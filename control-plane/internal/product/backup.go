package product

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

const backupArchiveLimit int64 = 512 << 20
const backupExpandedLimit int64 = 1 << 30
const backupFileLimit int64 = 64 << 20
const backupEntryLimit = 20000

type backupEntry struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	Executable bool   `json:"executable,omitempty"`
}
type backupManifest struct {
	Version   int           `json:"version"`
	ProjectID string        `json:"projectId"`
	Name      string        `json:"name"`
	Head      string        `json:"head"`
	Branch    string        `json:"branch"`
	CreatedAt string        `json:"createdAt"`
	Entries   []backupEntry `json:"entries"`
	Excluded  []string      `json:"excluded"`
	Notice    string        `json:"notice"`
}

func (s *Server) registerBackups(mux Registrar) {
	mux.HandleFunc("POST /v1/product/projects/{id}/backups", s.exportProjectBackup)
	mux.HandleFunc("POST /v1/product/projects/restore", s.restoreProjectBackup)
}

func backupSafePath(name string) bool {
	if name == "" || len(name) > 1024 || strings.ContainsAny(name, "\\:\x00\r\n") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
		for _, c := range part {
			if c < 32 || strings.ContainsRune(`<>"|?*`, c) {
				return false
			}
		}
	}
	return true
}
func validBackupEntry(name string) bool {
	if name == "git.bundle" || name == "journey.json" {
		return true
	}
	if !strings.HasPrefix(name, "files/") {
		return false
	}
	path := strings.TrimPrefix(name, "files/")
	return backupSafePath(path) && !blockedProjectFile(path)
}

func (s *Server) exportProjectBackup(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery(p.ID) {
		problem(w, 409, "delivery_busy", "Another delivery or backup action is running for this project.")
		return
	}
	defer s.releaseDelivery(p.ID)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	tmp, err := os.MkdirTemp(s.stateDir, ".backup-")
	if err != nil {
		problem(w, 500, "backup_failed", "Could not prepare the backup.")
		return
	}
	defer os.RemoveAll(tmp)
	archive, err := s.buildProjectBackup(ctx, p, tmp)
	if err != nil {
		problem(w, 422, "backup_failed", err.Error())
		return
	}
	f, err := os.Open(archive)
	if err != nil {
		problem(w, 500, "backup_failed", "Could not read the completed backup.")
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-luci-backup.zip"`, p.Name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if info != nil {
		w.Header().Set("Content-Length", fmt.Sprint(info.Size()))
	}
	w.WriteHeader(200)
	_, _ = io.Copy(w, f)
}

func (s *Server) buildProjectBackup(ctx context.Context, p project.Project, tmp string) (string, error) {
	manifest := backupManifest{Version: 1, ProjectID: p.ID, Name: p.Name, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Entries: []backupEntry{}, Excluded: []string{}, Notice: "Private project backup. Git history may contain previously committed secrets. Current credential-like files, ignored/generated files, Git configuration/hooks, host credentials, sessions and application databases/runtime volumes are excluded. Staging is restored as working changes; reconnect the remote after restore."}
	branch, _ := s.journeyGit(ctx, p.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	manifest.Branch = strings.TrimSpace(branch)
	if manifest.Branch != "" && !validBackupBranch(manifest.Branch) {
		return "", errors.New("Rename this branch to a portable Git branch name before exporting.")
	}
	head, headErr := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	if headErr == nil {
		manifest.Head = strings.TrimSpace(head)
		if !deliverySHA.MatchString(manifest.Head) {
			return "", errors.New("Git returned an invalid commit ID.")
		}
	}
	before, err := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return "", errors.New("Could not read project Git state.")
	}
	paths, err := s.journeyGit(ctx, p.Path, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil || len(paths) >= 2<<20 {
		return "", errors.New("Could not enumerate project files, or the file listing exceeds 2 MiB.")
	}
	stages, err := s.journeyGit(ctx, p.Path, "ls-files", "--stage", "-z")
	if err != nil || len(stages) >= 2<<20 {
		return "", errors.New("Could not inspect the Git index.")
	}
	for _, line := range strings.Split(stages, "\x00") {
		if strings.HasPrefix(line, "160000 ") {
			return "", errors.New("Submodule contents need their own project backup. Export is unavailable while this project has submodules.")
		}
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return "", errors.New("Could not read saved brief and design history.")
	}
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(tmp, "journey.json"), stateBytes, 0600); err != nil {
		return "", err
	}
	type source struct {
		path, name string
		max        int64
	}
	sources := []source{{filepath.Join(tmp, "journey.json"), "journey.json", 8 << 20}}
	if manifest.Head != "" {
		bundle := filepath.Join(tmp, "git.bundle")
		if _, err = s.journeyGit(ctx, p.Path, "bundle", "create", bundle, "--all", "HEAD"); err != nil {
			return "", errors.New("Git could not create a complete history bundle.")
		}
		sources = append(sources, source{bundle, "git.bundle", backupArchiveLimit})
	}
	seen := map[string]bool{}
	for _, name := range strings.Split(paths, "\x00") {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if blockedProjectFile(name) {
			manifest.Excluded = append(manifest.Excluded, name)
			continue
		}
		if !backupSafePath(name) {
			return "", errors.New("A project file has a non-portable path. Rename it before exporting.")
		}
		path, err := guardedProjectPath(p.Path, name, false)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", errors.New("A project file links outside the project or has an unsafe path.")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("Only ordinary project files can be backed up. Remove symbolic links or special files before exporting.")
		}
		sources = append(sources, source{path, "files/" + name, backupFileLimit})
		if len(sources) > backupEntryLimit {
			return "", errors.New("This backup exceeds 20,000 files.")
		}
	}
	sort.Strings(manifest.Excluded)
	archive := filepath.Join(tmp, "backup.zip")
	f, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	z := zip.NewWriter(f)
	defer f.Close()
	var total int64
	for _, src := range sources {
		if ctx.Err() != nil {
			return "", errors.New("Backup timed out or was cancelled.")
		}
		info, err := os.Lstat(src.path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > src.max {
			return "", errors.New("A backup entry is unavailable or exceeds its size limit (64 MiB per project file, 512 MiB Git history).")
		}
		total += info.Size()
		if total > backupExpandedLimit {
			return "", errors.New("The expanded backup exceeds 1 GiB.")
		}
		in, err := os.Open(src.path)
		if err != nil {
			return "", err
		}
		opened, err := in.Stat()
		if err != nil || !os.SameFile(info, opened) {
			in.Close()
			return "", errors.New("A project file changed during backup. Retry once editing has stopped.")
		}
		header := &zip.FileHeader{Name: src.name, Method: zip.Deflate}
		header.SetMode(0600)
		out, err := z.CreateHeader(header)
		if err != nil {
			in.Close()
			return "", err
		}
		hash := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, src.max+1))
		after, statErr := in.Stat()
		in.Close()
		if copyErr != nil || statErr != nil || n != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return "", errors.New("A project file changed during backup. Retry once editing has stopped.")
		}
		manifest.Entries = append(manifest.Entries, backupEntry{Path: src.name, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: n, Executable: info.Mode()&0111 != 0})
	}
	ending, err := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	newHead, _ := s.journeyGit(ctx, p.Path, "rev-parse", "--verify", "HEAD")
	if err != nil || ending != before || (manifest.Head != "" && strings.TrimSpace(newHead) != manifest.Head) {
		return "", errors.New("Project Git state changed during backup. Retry once editing has stopped.")
	}
	meta, err := z.Create("manifest.json")
	if err != nil {
		return "", err
	}
	if err = json.NewEncoder(meta).Encode(manifest); err != nil {
		return "", err
	}
	if err = z.Close(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	info, err := os.Stat(archive)
	if err != nil || info.Size() > backupArchiveLimit {
		return "", errors.New("The compressed backup exceeds 512 MiB.")
	}
	return archive, nil
}

func validBackupBranch(branch string) bool {
	return deliveryBranch.MatchString(branch) && !strings.Contains(branch, "..") && !strings.Contains(branch, "//") && !strings.HasSuffix(branch, "/") && !strings.HasSuffix(branch, ".") && !strings.HasSuffix(branch, ".lock") && !strings.Contains(branch, "/.")
}

func readBackupArchive(path, dir string) (backupManifest, error) {
	var manifest backupManifest
	z, err := zip.OpenReader(path)
	if err != nil {
		return manifest, errors.New("Upload a valid Luci ZIP backup.")
	}
	defer z.Close()
	if len(z.File) < 2 || len(z.File) > backupEntryLimit+1 {
		return manifest, errors.New("The backup has too many entries or is incomplete.")
	}
	seen := map[string]*zip.File{}
	var total uint64
	for _, f := range z.File {
		key := strings.ToLower(f.Name)
		if seen[key] != nil || !backupSafePath(f.Name) || (f.Name != "manifest.json" && !validBackupEntry(f.Name)) || !f.Mode().IsRegular() || f.Flags&1 != 0 || (f.Method != zip.Deflate && f.Method != zip.Store) {
			return manifest, errors.New("The archive contains a duplicate, protected or unsafe entry.")
		}
		limit := backupFileLimit
		if f.Name == "git.bundle" {
			limit = backupArchiveLimit
		}
		if f.Name == "journey.json" {
			limit = 8 << 20
		}
		if f.Name == "manifest.json" {
			limit = 8 << 20
		}
		if f.UncompressedSize64 > uint64(limit) {
			return manifest, errors.New("A backup entry exceeds its size limit.")
		}
		total += f.UncompressedSize64
		if total > uint64(backupExpandedLimit) {
			return manifest, errors.New("The expanded backup exceeds 1 GiB.")
		}
		seen[key] = f
	}
	mf := seen["manifest.json"]
	if mf == nil || mf.Name != "manifest.json" {
		return manifest, errors.New("The backup manifest is missing.")
	}
	in, err := mf.Open()
	if err != nil {
		return manifest, err
	}
	data, err := io.ReadAll(io.LimitReader(in, (8<<20)+1))
	in.Close()
	if err != nil || len(data) > 8<<20 || json.Unmarshal(data, &manifest) != nil {
		return manifest, errors.New("The backup manifest is invalid.")
	}
	if manifest.Version != 1 || !validProjectName(manifest.Name) || manifest.ProjectID == "" || (manifest.Head != "" && !deliverySHA.MatchString(manifest.Head)) || (manifest.Branch != "" && !validBackupBranch(manifest.Branch)) || len(manifest.Entries) != len(z.File)-1 {
		return manifest, errors.New("The backup format or project metadata is unsupported.")
	}
	listed := map[string]bool{}
	for _, entry := range manifest.Entries {
		f := seen[strings.ToLower(entry.Path)]
		if f == nil || f.Name != entry.Path || listed[entry.Path] || !validBackupEntry(entry.Path) || entry.Size < 0 || uint64(entry.Size) != f.UncompressedSize64 || len(entry.SHA256) != 64 {
			return manifest, errors.New("The backup inventory does not match the archive.")
		}
		listed[entry.Path] = true
		dest := filepath.Join(dir, filepath.FromSlash(entry.Path))
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return manifest, err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return manifest, errors.New("The backup contains conflicting paths.")
		}
		in, err := f.Open()
		if err != nil {
			out.Close()
			return manifest, err
		}
		hash := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, entry.Size+1))
		closeErr := out.Close()
		in.Close()
		if copyErr != nil || closeErr != nil || n != entry.Size || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
			return manifest, errors.New("A backup entry failed its size, CRC or SHA-256 integrity check.")
		}
	}
	if !listed["journey.json"] || (manifest.Head != "") != listed["git.bundle"] {
		return manifest, errors.New("The backup is missing its project history.")
	}
	return manifest, nil
}

func (s *Server) restoreProjectBackup(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if !validProjectName(name) {
		problem(w, 400, "invalid_name", "Choose a new project name using 1–64 letters, digits, hyphens or underscores.")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "application/zip" && ct != "application/octet-stream" {
		problem(w, 415, "invalid_content_type", "Upload the backup as application/zip.")
		return
	}
	tmp, err := os.MkdirTemp(s.stateDir, ".restore-")
	if err != nil {
		problem(w, 500, "restore_failed", "Could not prepare restore.")
		return
	}
	defer os.RemoveAll(tmp)
	input := filepath.Join(tmp, "input.zip")
	f, err := os.OpenFile(input, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		problem(w, 500, "restore_failed", "Could not prepare the uploaded backup.")
		return
	}
	n, copyErr := io.Copy(f, io.LimitReader(r.Body, backupArchiveLimit+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n > backupArchiveLimit {
		problem(w, 413, "backup_too_large", "The uploaded backup must be at most 512 MiB.")
		return
	}
	extracted := filepath.Join(tmp, "contents")
	if err = os.Mkdir(extracted, 0700); err != nil {
		problem(w, 500, "restore_failed", "Could not prepare restore validation.")
		return
	}
	manifest, err := readBackupArchive(input, extracted)
	if err != nil {
		problem(w, 400, "invalid_backup", err.Error())
		return
	}
	data, err := os.ReadFile(filepath.Join(extracted, "journey.json"))
	var state journeyState
	if err != nil || json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Brief.ProjectID != manifest.ProjectID || state.Design.ProjectID != manifest.ProjectID || state.Brief.Revision < 0 || state.Design.Revision < 0 {
		problem(w, 400, "invalid_history", "The saved brief/design history does not match this backup.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	root := s.projectRoot()
	staged, err := os.MkdirTemp(root, ".luci-restore-")
	if err != nil {
		problem(w, 500, "restore_failed", "Could not prepare the new project.")
		return
	}
	defer os.RemoveAll(staged)
	if _, err = s.journeyGit(ctx, staged, "-c", "init.templateDir=", "init", "--initial-branch=main", "."); err != nil {
		problem(w, 422, "restore_failed", "Git could not initialise the restored project.")
		return
	}
	if manifest.Head != "" {
		bundle := filepath.Join(extracted, "git.bundle")
		if _, err = s.journeyGit(ctx, staged, "bundle", "verify", bundle); err != nil {
			problem(w, 400, "invalid_git_history", "The Git bundle is incomplete or invalid.")
			return
		}
		if _, err = s.journeyGit(ctx, staged, "-c", "protocol.allow=never", "-c", "protocol.file.allow=always", "fetch", "--update-head-ok", "--no-tags", bundle, "+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*", "HEAD"); err != nil {
			problem(w, 400, "invalid_git_history", "Git could not restore the bundle's branches and tags.")
			return
		}
	}
	if manifest.Branch != "" {
		if _, err = s.journeyGit(ctx, staged, "symbolic-ref", "HEAD", "refs/heads/"+manifest.Branch); err != nil {
			problem(w, 400, "invalid_git_history", "Git could not restore the branch.")
			return
		}
	}
	if manifest.Head != "" {
		if _, err = s.journeyGit(ctx, staged, "cat-file", "-e", manifest.Head+"^{commit}"); err != nil {
			problem(w, 400, "invalid_git_history", "The saved commit is missing from the bundle.")
			return
		}
		// Prepare the index without checking out potentially hostile tree paths.
		if manifest.Branch == "" {
			_, err = s.journeyGit(ctx, staged, "update-ref", "--no-deref", "HEAD", manifest.Head)
		} else {
			_, err = s.journeyGit(ctx, staged, "update-ref", "HEAD", manifest.Head)
		}
		if err != nil {
			problem(w, 400, "invalid_git_history", "Git could not restore HEAD.")
			return
		}
		if _, err = s.journeyGit(ctx, staged, "read-tree", manifest.Head); err != nil {
			problem(w, 400, "invalid_git_history", "Git could not restore the saved index.")
			return
		}
	}
	dest := filepath.Join(root, name)
	// Mkdir is an atomic no-overwrite reservation, even against a symlink.
	if err = os.Mkdir(dest, 0700); err != nil {
		problem(w, 409, "project_exists", "A file or project already uses this name, or the projects folder is unavailable.")
		return
	}
	newID := project.ProjectID(dest)
	state.Brief.ProjectID = newID
	state.Design.ProjectID = newID
	if state.DesignPlan != nil {
		state.DesignPlan.ProjectID = newID
		state.DesignPlan.Status = "draft"
		state.DesignPlan.Digest = ""
		state.DesignPlan.PrototypeReleaseID = ""
		state.DesignPlan.PrototypeCommit = ""
	}
	for i := range state.DesignPlanRevisions {
		state.DesignPlanRevisions[i].ProjectID = newID
		state.DesignPlanRevisions[i].Status = "draft"
		state.DesignPlanRevisions[i].Digest = ""
		state.DesignPlanRevisions[i].PrototypeReleaseID = ""
		state.DesignPlanRevisions[i].PrototypeCommit = ""
	}
	if state.Specification != nil {
		state.Specification.ProjectID = newID
		state.Specification.Status = "draft"
		state.Specification.Digest = ""
	}
	for i := range state.SpecificationRevisions {
		state.SpecificationRevisions[i].ProjectID = newID
		// A restored project has a new identity; the former approval cannot
		// attest to this new source/environment. Preserve the content history.
		state.SpecificationRevisions[i].Status = "draft"
		state.SpecificationRevisions[i].Digest = ""
	}
	if state.Plan != nil {
		state.Plan.ProjectID = newID
	}
	for i := range state.BriefRevisions {
		state.BriefRevisions[i].ProjectID = newID
	}
	for i := range state.DesignRevisions {
		state.DesignRevisions[i].ProjectID = newID
	}
	for _, entry := range manifest.Entries {
		if !strings.HasPrefix(entry.Path, "files/") {
			continue
		}
		rel := strings.TrimPrefix(entry.Path, "files/")
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err = os.MkdirAll(filepath.Dir(target), 0700); err == nil {
			err = copyBackupFile(filepath.Join(extracted, filepath.FromSlash(entry.Path)), target)
		}
		if err == nil && entry.Executable {
			err = os.Chmod(target, 0700)
		}
		if err != nil {
			problem(w, 500, "restore_failed", "Restore could not finish. Its new folder was retained for inspection; existing projects were unchanged.")
			return
		}
	}
	if err = writeJourney(dest, state); err == nil && state.Brief.Revision > 0 {
		err = writeProjectAtomic(dest, "LUCI.md", renderBrief(state.Brief))
	}
	if err == nil {
		err = os.Rename(filepath.Join(staged, ".git"), filepath.Join(dest, ".git"))
	}
	if err != nil {
		problem(w, 500, "restore_failed", "Restore could not finish. Its new folder was retained for inspection; existing projects were unchanged.")
		return
	}
	// The .git directory is installed last, so incomplete restores are not
	// catalogued as projects. Credentials and remotes are never imported.
	writeJSON(w, 201, project.Project{ID: newID, Name: name, Path: dest})
}

func copyBackupFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
