package platform

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

const dataBackupLimit = 64 << 20

type dataBackupMetadata struct {
	ProjectID       string   `json:"projectId"`
	Version         string   `json:"version"`
	DatabaseSHA256  string   `json:"databaseSha256"`
	StorageSHA256   string   `json:"storageSha256"`
	StorageTenantID string   `json:"storageTenantId,omitempty"`
	RealtimeTables  []string `json:"realtimeTables"`
}

func (s *Service) dataVolumeCommand(ctx context.Context, i dataInstallation, write bool, input []byte) ([]byte, error) {
	m, e := reviewedDataAssets()
	if e != nil {
		return nil, e
	}
	image := m.Images["supabase/postgres:17.6.1.136"]
	volume := s.resource(i.ProjectID, i.StackID+"-storage-data")
	b, e := s.docker(ctx, "volume", "inspect", volume, "--format", "{{json .Labels}}")
	if e != nil {
		return nil, e
	}
	var labels map[string]string
	if json.Unmarshal(b, &labels) != nil || labels[labelInstance] != s.instance || labels[labelProject] != i.ProjectID || labels[labelResource] != "data-"+i.StackID {
		return nil, errors.New("data volume ownership mismatch")
	}
	mount := "type=volume,source=" + volume + ",target=/backup"
	if !write {
		mount += ",readonly"
	}
	args := []string{"run", "--rm", "--interactive", "--network", "none", "--read-only", "--memory", "128m", "--memory-swap", "128m", "--cpus", "0.25", "--pids-limit", "64", "--user", "0", "--mount", mount}
	args = append(args, s.labels(i.ProjectID, "data-"+i.StackID+"-backup")...)
	args = append(args, "--entrypoint", "tar", image)
	if write {
		args = append(args, "--numeric-owner", "-xzf", "-", "-C", "/backup")
	} else {
		args = append(args, "--numeric-owner", "-czf", "-", "-C", "/backup", ".")
	}
	return s.opts.Runner(ctx, "docker", args, s.dir, input)
}
func validateStorageArchive(b []byte) error {
	if len(b) > dataBackupLimit {
		return errors.New("storage backup exceeds limit")
	}
	reader, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		return e
	}
	defer reader.Close()
	tr := tar.NewReader(io.LimitReader(reader, 512<<20))
	count := 0
	total := int64(0)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		name := strings.TrimPrefix(h.Name, "./")
		// GNU tar writes directory headers with a trailing slash. Retain the
		// canonical-path and link checks while accepting that normal form.
		if h.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if name == "." || name == "" {
			if h.Typeflag == tar.TypeDir {
				continue
			}
			return errors.New("invalid root archive entry")
		}
		if path.Clean(name) != name || path.IsAbs(name) || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00\r\n") || len(name) > 1024 {
			return errors.New("unsafe storage archive path")
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return errors.New("storage archive contains links or special files")
		}
		count++
		total += h.Size
		if count > 100000 || total > 512<<20 {
			return errors.New("expanded storage archive exceeds limit")
		}
		if _, e = io.Copy(io.Discard, tr); e != nil {
			return e
		}
	}
}
func (s *Service) backupData(ctx context.Context, i dataInstallation) error {
	if e := s.verifyDataOwnership(ctx, i, false); e != nil {
		return e
	}
	s.dataStage(i.ProjectID, "pausing", "Pausing the data API while taking a database and file backup. Direct database clients must pause their writes separately.")
	paused := []string{}
	for _, name := range dataServiceNames {
		if name != "db" {
			paused = append(paused, name)
		}
	}
	if _, e := s.compose(ctx, i, append([]string{"stop", "--timeout", "30"}, paused...)...); e != nil {
		return e
	}
	defer func() {
		_, _ = s.compose(ctx, i, "up", "-d", "--wait", "--wait-timeout", "240")
		_ = s.refreshData(ctx, i, false)
	}()
	s.dataStage(i.ProjectID, "dumping", "Backing up public tables, Auth and Storage metadata, and storage files.")
	dump, e := s.docker(ctx, "exec", s.dataName(i, "db"), "pg_dump", "-U", "postgres", "-d", "postgres", "--format=custom", "--no-owner", "--schema=public", "--schema=auth", "--schema=storage")
	if e != nil || len(dump) < 5 || string(dump[:5]) != "PGDMP" {
		return errors.New("invalid Supabase database backup")
	}
	files, e := s.dataVolumeCommand(ctx, i, false, nil)
	if e != nil {
		return e
	}
	if e = validateStorageArchive(files); e != nil {
		return e
	}
	tables, e := jsonSQL(ctx, s, i, `SELECT COALESCE(json_agg(tablename),'[]'::json) FROM pg_publication_tables WHERE pubname='supabase_realtime' AND schemaname='public';`)
	if e != nil {
		return e
	}
	tableBytes, _ := json.Marshal(tables)
	var realtimeTables []string
	if json.Unmarshal(tableBytes, &realtimeTables) != nil {
		return errors.New("invalid publication backup")
	}
	meta := dataBackupMetadata{ProjectID: i.ProjectID, Version: "self-hosted/v0.8.2", DatabaseSHA256: dataSum(dump), StorageSHA256: dataSum(files), StorageTenantID: i.storageTenant(), RealtimeTables: realtimeTables}
	metadata, _ := json.Marshal(meta)
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, entry := range []struct {
		name string
		body []byte
	}{{"metadata.json", metadata}, {"database.dump", dump}, {"storage.tar.gz", files}} {
		f, e := zw.Create(entry.name)
		if e != nil {
			return e
		}
		if _, e = f.Write(entry.body); e != nil {
			return e
		}
	}
	if e = zw.Close(); e != nil {
		return e
	}
	if out.Len() > dataBackupLimit {
		return &dataFailure{"This database and file backup exceeds Luci's 64 MiB backup limit. Use an external backup pipeline for larger data."}
	}
	b := DataBackup{ID: randomID("data-backup-"), CreatedAt: now(), Bytes: int64(out.Len()), SHA256: dataSum(out.Bytes()), IncludesStorage: true}
	file, e := s.path(b.ID)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if e = protect(file); e == nil {
		_, e = f.Write(out.Bytes())
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		_ = os.Remove(file)
		return e
	}
	// Restore encryption/signing keys are private companion state. Downloaded
	// backups contain application data, not a service-role token or DB password.
	secret, e := s.dataCredentials(i)
	if e != nil {
		return e
	}
	if e = s.write(b.ID+"-secret", secret); e != nil {
		return e
	}
	return s.changeData(i.ProjectID, func(v *DataStatus) {
		v.Backups = append([]DataBackup{b}, v.Backups...)
		v.Capabilities["restore"] = true
		v.Message = "Database and file backup recorded with a verified checksum. Data APIs are restarting; direct database writes were not automatically paused."
	})
}
func (s *Service) readDataBackup(id, backupID string) (DataBackup, []byte, error) {
	s.mu.Lock()
	state, e := s.loadData(id)
	s.mu.Unlock()
	if e != nil {
		return DataBackup{}, nil, e
	}
	var selected DataBackup
	for _, b := range state.Backups {
		if b.ID == backupID {
			selected = b
			break
		}
	}
	if selected.ID == "" {
		return selected, nil, os.ErrNotExist
	}
	file, e := s.path(selected.ID)
	if e != nil {
		return selected, nil, e
	}
	f, e := os.Open(file)
	if e != nil {
		return selected, nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, dataBackupLimit+1))
	if e != nil || int64(len(b)) != selected.Bytes || len(b) > dataBackupLimit || dataSum(b) != selected.SHA256 {
		return selected, nil, errors.New("backup checksum changed")
	}
	return selected, b, nil
}
func (s *Service) downloadDataBackup(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	selected, b, e := s.readDataBackup(p.ID, r.PathValue("backupId"))
	if e != nil {
		problem(w, 404, "backup_unavailable", "Choose an intact data backup recorded in this project.")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `attachment; filename="`+selected.ID+`.zip"`)
	_, _ = w.Write(b)
}
func unpackDataBackup(id string, b []byte) (dataBackupMetadata, []byte, []byte, error) {
	var meta dataBackupMetadata
	zr, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return meta, nil, nil, e
	}
	entries := map[string][]byte{}
	for _, f := range zr.File {
		if (f.Name != "metadata.json" && f.Name != "database.dump" && f.Name != "storage.tar.gz") || entries[f.Name] != nil || f.UncompressedSize64 > dataBackupLimit {
			return meta, nil, nil, errors.New("unexpected data backup entry")
		}
		r, e := f.Open()
		if e != nil {
			return meta, nil, nil, e
		}
		entry, e := io.ReadAll(io.LimitReader(r, dataBackupLimit+1))
		_ = r.Close()
		if e != nil || len(entry) > dataBackupLimit {
			return meta, nil, nil, errors.New("backup entry exceeds limit")
		}
		entries[f.Name] = entry
	}
	if len(entries) != 3 || json.Unmarshal(entries["metadata.json"], &meta) != nil || meta.ProjectID != id || meta.Version != "self-hosted/v0.8.2" {
		return meta, nil, nil, errors.New("backup provenance mismatch")
	}
	if meta.StorageTenantID != "" && !identifier.MatchString(meta.StorageTenantID) {
		return meta, nil, nil, errors.New("invalid storage namespace")
	}
	dump, files := entries["database.dump"], entries["storage.tar.gz"]
	if len(dump) < 5 || string(dump[:5]) != "PGDMP" || dataSum(dump) != meta.DatabaseSHA256 || dataSum(files) != meta.StorageSHA256 {
		return meta, nil, nil, errors.New("backup content checksum mismatch")
	}
	if e = validateStorageArchive(files); e != nil {
		return meta, nil, nil, e
	}
	for _, table := range meta.RealtimeTables {
		if !dataNamePattern.MatchString(table) {
			return meta, nil, nil, errors.New("invalid publication identifier")
		}
	}
	return meta, dump, files, nil
}

func ownedDataStorageTenant(tenant string, i dataInstallation) bool {
	if !identifier.MatchString(tenant) {
		return false
	}
	if tenant == i.storageTenant() || tenant == i.StackID {
		return true
	}
	for _, previous := range i.Previous {
		if tenant == previous {
			return true
		}
	}
	return false
}

// Older backups did not record the tenant used by Storage's file backend.
// Recover it only from one consistent namespace belonging to this project's
// active or retained installation; empty volumes use the existing namespace.
func legacyDataStorageTenant(files []byte, i dataInstallation) (string, error) {
	reader, e := gzip.NewReader(bytes.NewReader(files))
	if e != nil {
		return "", e
	}
	defer reader.Close()
	tr := tar.NewReader(io.LimitReader(reader, 512<<20))
	tenant := ""
	for {
		h, e := tr.Next()
		if e == io.EOF {
			if tenant == "" {
				tenant = i.storageTenant()
			}
			return tenant, nil
		}
		if e != nil {
			return "", e
		}
		name := strings.Trim(strings.TrimPrefix(h.Name, "./"), "/")
		if name == "" || name == "." {
			continue
		}
		// The reviewed file backend uses GLOBAL_S3_BUCKET=luci, followed
		// by the tenant namespace; the top bucket directory carries no data.
		if name == "luci" && h.Typeflag == tar.TypeDir {
			continue
		}
		relative, found := strings.CutPrefix(name, "luci/")
		if !found {
			return "", errors.New("legacy backup has an unexpected storage root")
		}
		candidate, _, _ := strings.Cut(relative, "/")
		if !ownedDataStorageTenant(candidate, i) || (tenant != "" && tenant != candidate) {
			return "", errors.New("legacy backup storage namespace is not an owned installation")
		}
		tenant = candidate
	}
}

func (s *Service) detachDataNetwork(ctx context.Context, i dataInstallation) error {
	network := s.resource(i.ProjectID, "net")
	for _, name := range []string{"api-gw", "db"} {
		if _, e := s.inspect(ctx, i.ProjectID, "data-"+i.StackID+"-"+name, s.dataName(i, name)); e != nil {
			return e
		}
		b, e := s.docker(ctx, "inspect", s.dataName(i, name), "--format", "{{json .NetworkSettings.Networks}}")
		if e != nil {
			return e
		}
		var attached map[string]json.RawMessage
		if json.Unmarshal(b, &attached) != nil {
			return errors.New("invalid network response")
		}
		if _, ok := attached[network]; ok {
			if _, e = s.docker(ctx, "network", "disconnect", network, s.dataName(i, name)); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *Service) restoreData(ctx context.Context, id string, previous dataInstallation, backupID string) (err error) {
	_, b, e := s.readDataBackup(id, backupID)
	if e != nil {
		return &dataFailure{"This backup is unavailable or no longer matches its recorded checksum."}
	}
	meta, dump, files, e := unpackDataBackup(id, b)
	if e != nil {
		return e
	}
	tenant := meta.StorageTenantID
	if tenant == "" {
		tenant, e = legacyDataStorageTenant(files, previous)
		if e != nil {
			return e
		}
	}
	if !ownedDataStorageTenant(tenant, previous) {
		return errors.New("backup storage namespace does not belong to this installation")
	}
	if e = s.dataCapacity(ctx, id, true); e != nil {
		return e
	}
	var secret dataSecret
	if e = s.read(backupID+"-secret", &secret); e != nil {
		return &dataFailure{"The private restore keys for this backup are unavailable. Original database and storage backups are retained."}
	}
	next := dataInstallation{ProjectID: id, StackID: randomID("stack-"), StorageTenantID: tenant, Previous: append([]string{}, previous.Previous...)}
	if previous.StackID != "" {
		next.Previous = append(next.Previous, previous.StackID)
	}
	next.APIPort, e = freeDataPort()
	if e != nil {
		return e
	}
	if e = s.write(next.StackID+"-secret", secret); e != nil {
		return e
	}
	if e = s.prepareDataStack(next, secret); e != nil {
		return e
	}
	candidateStarted := false
	switched := false
	previousStopped := false
	defer func() {
		if err == nil || switched {
			return
		}
		if candidateStarted {
			for _, name := range dataServiceNames {
				container := s.dataName(next, name)
				if info, e := s.inspect(ctx, id, "data-"+next.StackID+"-"+name, container); e == nil && info.State.Running {
					_, _ = s.docker(ctx, "stop", container)
				}
			}
			var retained []string
			_ = s.read(id+"-data-retained", &retained)
			retained = append(retained, next.StackID)
			_ = s.write(id+"-data-retained", retained)
			_ = s.changeData(id, func(v *DataStatus) { v.PriorStackRetained = true })
		}
		if previousStopped && previous.StackID != "" {
			_, _ = s.compose(ctx, previous, "up", "-d", "--wait", "--wait-timeout", "240")
			_ = s.attachDataNetwork(ctx, previous)
			_ = s.write(id+"-data-installation", previous)
		}
	}()
	s.dataStage(id, "restoring", "Restoring into a fresh owned PostgreSQL 17 and file volume. The existing stack stays available until the restored stack passes checks.")
	if e = s.verifyDataOwnership(ctx, next, true); e != nil {
		return e
	}
	candidateStarted = true
	if _, e = s.compose(ctx, next, "up", "-d", "--wait", "--wait-timeout", "180", "db"); e != nil {
		return e
	}
	// A fresh Supabase database already contains its bootstrap schemas and
	// functions, but not the application's tables. Cleaning every archive item
	// tries DROP POLICY/CONSTRAINT ON those absent tables; IF EXISTS applies to
	// the policy, not its parent relation. Clean and recreate pre-data objects
	// first, then load data and post-data without dropping their absent parents.
	// Each phase is atomic, and neither phase touches the active installation.
	// The image's postgres role is intentionally not a superuser: bootstrap
	// Auth tables belong to supabase_auth_admin. Use the image's local bootstrap
	// administrator and preserve archive ownership so Auth/Storage retain the
	// service roles' migration and maintenance privileges.
	for _, phase := range [][]string{
		{"--section=pre-data", "--clean", "--if-exists"},
		{"--section=data", "--section=post-data"},
	} {
		args := []string{"exec", "--interactive", s.dataName(next, "db"), "pg_restore", "-U", "supabase_admin", "-d", "postgres", "--single-transaction"}
		args = append(args, phase...)
		if _, e = s.opts.Runner(ctx, "docker", args, s.dir, dump); e != nil {
			return e
		}
	}
	// Compose creates volumes lazily. Create only the missing owned Storage
	// volume, labelled exactly as the generated Compose configuration expects.
	volume := s.resource(id, next.StackID+"-storage-data")
	args := append([]string{"volume", "create"}, s.labels(id, "data-"+next.StackID)...)
	args = append(args, volume)
	if _, e = s.docker(ctx, args...); e != nil {
		return e
	}
	if _, e = s.dataVolumeCommand(ctx, next, true, files); e != nil {
		return e
	}
	for _, table := range meta.RealtimeTables {
		if _, e = s.dataSQL(ctx, next, "ALTER PUBLICATION supabase_realtime ADD TABLE public."+sqlIdentifier(table)+";"); e != nil {
			return e
		}
	}
	if _, e = s.compose(ctx, next, "up", "-d", "--wait", "--wait-timeout", "240"); e != nil {
		return e
	}
	if _, e = s.dataSQL(ctx, next, "SELECT 1;"); e != nil {
		return e
	}
	for _, path := range []string{"/auth/v1/health", "/rest/v1/", "/storage/v1/status"} {
		if _, e = s.dataHTTP(ctx, next, secret, "GET", path, nil, ""); e != nil {
			return e
		}
	}
	s.dataStage(id, "switching", "Restored services passed checks. Switching private application bindings and retaining the previous stack for recovery.")
	if previous.StackID != "" {
		if e = s.verifyDataOwnership(ctx, previous, false); e != nil {
			return e
		}
		if _, e = s.compose(ctx, previous, "stop", "--timeout", "30"); e != nil {
			return e
		}
		previousStopped = true
		if e = s.detachDataNetwork(ctx, previous); e != nil {
			return e
		}
	}
	if e = s.attachDataNetwork(ctx, next); e != nil {
		return e
	}
	if e = s.write(id+"-data-installation", next); e != nil {
		return e
	}
	if e = s.refreshData(ctx, next, true); e != nil {
		return e
	}
	switched = true
	return s.changeData(id, func(v *DataStatus) {
		v.PriorStackRetained = previous.StackID != ""
		v.Message = "Restored database and storage passed health checks. The previous stack is stopped and retained. Redeploy applications to update their client-facing data URL."
	})
}
