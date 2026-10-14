package product

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const maxProjectFile = 1 << 20

func blockedProjectFile(relative string) bool {
	for _, component := range strings.Split(strings.ToLower(strings.ReplaceAll(relative, "\\", "/")), "/") {
		switch component {
		case ".git", ".luci", ".ssh", ".aws", ".azure", ".kube", ".gnupg", ".env", ".netrc", ".npmrc", ".pypirc", ".dockercfg", "credentials", "credentials.json", "secrets.json", "id_rsa", "id_ed25519", "id_dsa", "id_ecdsa":
			return true
		}
		if strings.HasPrefix(component, ".env.") || strings.HasSuffix(component, ".pem") || strings.HasSuffix(component, ".key") || strings.HasSuffix(component, ".p12") || strings.HasSuffix(component, ".pfx") {
			return true
		}
	}
	return false
}

// Only the documented context exports may cross the one-click Git boundary.
// Other private Luci files remain protected even when stored beside them.
func journeyContextFile(relative string) bool {
	if journeyArchivePath(relative) {
		return true
	}
	switch relative {
	case ".luci/state.json", ".luci/design.json", ".luci/plan.json",
		".luci/specification.json", ".luci/specification.md",
		".luci/design-handoff.json", ".luci/design-handoff.md":
		return true
	}
	return false
}

func (s *Server) projectFiles(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	relative := r.URL.Query().Get("path")
	if blockedProjectFile(relative) {
		journeyError(w, 403, "protected_file", "Git internals, Luci state and credential files are not exposed by the file browser.")
		return
	}
	path, err := guardedProjectPath(p.Path, relative, false)
	if err != nil {
		journeyError(w, 403, "unsafe_path", err.Error())
		return
	}
	before, err := os.Lstat(path)
	if err != nil {
		journeyError(w, 404, "file_missing", err.Error())
		return
	}
	if !before.IsDir() && !before.Mode().IsRegular() {
		journeyError(w, 403, "unsupported_file", "Only ordinary directories and text files can be read.")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		journeyError(w, 404, "file_unavailable", err.Error())
		return
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		journeyError(w, 409, "file_changed", "The file changed while it was being opened. Retry the request.")
		return
	}
	if before.IsDir() {
		entries, err := file.ReadDir(4097)
		if err != nil && !errors.Is(err, io.EOF) {
			journeyError(w, 500, "directory_unavailable", err.Error())
			return
		}
		if len(entries) > 4096 {
			journeyError(w, 413, "directory_too_large", "This directory contains more than 4,096 entries.")
			return
		}
		type entry struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Kind string `json:"kind"`
			Size int64  `json:"size"`
		}
		out := []entry{}
		for _, item := range entries {
			rel := item.Name()
			if relative != "" && relative != "." {
				rel = relative + "/" + rel
			}
			if blockedProjectFile(rel) {
				continue
			}
			info, err := item.Info()
			if err != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
				continue
			}
			kind := "file"
			if info.IsDir() {
				kind = "directory"
			}
			out = append(out, entry{Name: item.Name(), Path: rel, Kind: kind, Size: info.Size()})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Kind != out[j].Kind {
				return out[i].Kind == "directory"
			}
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		})
		journeyJSON(w, 200, map[string]any{"path": relative, "entries": out})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProjectFile+1))
	if err != nil {
		journeyError(w, 500, "read_failed", err.Error())
		return
	}
	truncated := len(data) > maxProjectFile
	if truncated {
		data = data[:maxProjectFile]
		// A UTF-8 rune may straddle the truncation boundary.
		for i := 0; i < 3 && len(data) > 0 && !utf8.Valid(data); i++ {
			data = data[:len(data)-1]
		}
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		journeyError(w, 415, "binary_file", "The file browser displays UTF-8 text files.")
		return
	}
	journeyJSON(w, 200, map[string]any{"path": relative, "content": string(data), "truncated": truncated})
}

type GitChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
type GitLogEntry struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
}
type GitSummary struct {
	ReviewDigest string        `json:"reviewDigest"`
	Branch       string        `json:"branch"`
	Clean        bool          `json:"clean"`
	Changes      []GitChange   `json:"changes"`
	Log          []GitLogEntry `json:"log"`
	Diff         string        `json:"diff"`
}

// These flags keep read-only Git operations from invoking repository hooks,
// fsmonitor commands, pagers or external diff/textconv programs.
var journeyGitFlags = []string{"--no-pager", "--no-optional-locks", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "color.ui=false", "-c", "commit.gpgsign=false"}

// These exact commands read refs or index metadata. None asks Git to read a
// tracked file through a clean/smudge/process filter, so a separate Git
// filter-config probe before each one only starts another host process.
// Status, diff, staging, show and unknown commands keep the fail-closed probe.
func journeyGitMetadataOnly(command []string) bool {
	switch strings.Join(command, "\x00") {
	case "symbolic-ref\x00--quiet\x00HEAD",
		"symbolic-ref\x00--quiet\x00--short\x00HEAD",
		"rev-parse\x00--verify\x00--quiet\x00HEAD^{commit}",
		"rev-parse\x00--shared-index-path",
		"rev-parse\x00--show-object-format",
		"ls-files\x00--stage\x00-z",
		"ls-files\x00-v\x00-z",
		"ls-files\x00-z\x00--cached\x00--others\x00--exclude-standard":
		return true
	}
	return false
}

type managedJourneyGitContextKey struct{}
type journeyGitRootContextKey struct{}

// Callers already holding the journey mutex capture config.ProjectsDir under
// that lock. Git must not recursively acquire it, including an RLock when a
// configuration writer is waiting. Other callers resolve the root normally.
func (s *Server) journeyGitRoot(ctx context.Context) string {
	if root, ok := ctx.Value(journeyGitRootContextKey{}).(string); ok && root != "" {
		return root
	}
	return s.projectRoot()
}

// Keep the injectable command runner while applying the same environment to
// native execution and runner adapters that delegate back to RunCommand.
func journeyGitEnvironment(ctx context.Context, env []string) []string {
	if managed, _ := ctx.Value(managedJourneyGitContextKey{}).(bool); managed {
		env = project.ManagedGitEnvironment(env)
		if selected, ok := ctx.Value(privateReviewGitKey{}).(privateReviewGit); ok && selected.ObjectsDir != "" {
			env = append(env, "GIT_OBJECT_DIRECTORY="+selected.ObjectsDir)
		}
		return env
	}
	return env
}

func journeyGitBoundary(ctx context.Context, root, dir string, command []string) ([]string, error) {
	bounded, err := project.ManagedGitArguments(root, dir, command)
	if err != nil {
		return nil, err
	}
	if err := validateBranchMetadata(ctx, root, dir); err != nil {
		return nil, err
	}
	// A supported linked worktree has a .git file and keeps its actual metadata
	// in the primary repository's .git/worktrees directory. Validate that whole
	// common store too, including nested refs/objects/index files and hard links.
	if info, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
		return nil, err
	} else if !info.IsDir() {
		gitDir := strings.TrimPrefix(bounded[0], "--git-dir=")
		primary := filepath.Dir(filepath.Dir(filepath.Dir(gitDir)))
		if err := validateBranchMetadata(ctx, root, primary); err != nil {
			return nil, err
		}
	}
	return bounded, nil
}

func (s *Server) journeyGit(ctx context.Context, dir string, args ...string) (string, error) {
	root := s.journeyGitRoot(ctx)
	// Only controller-selected -c settings may precede the actual subcommand.
	// Resolve the boundary from that subcommand, then keep its location pins
	// after the settings so even a caller cannot redirect the managed worktree.
	settings, command := []string{}, args
	for len(command) >= 2 && command[0] == "-c" {
		settings = append(settings, command[:2]...)
		command = command[2:]
	}
	if len(command) == 0 || strings.HasPrefix(command[0], "-") {
		return "", errors.New("managed Git requires a direct subcommand")
	}
	// Explicit machine-level helper discovery remains outside repository
	// configuration. These narrow probes must retain the connected account's
	// trusted configuration, which managed repository commands never inherit.
	if len(settings) == 0 && len(command) == 5 && command[0] == "config" && (command[1] == "--system" || command[1] == "--global") && command[2] == "--get-urlmatch" && command[3] == "credential.helper" {
		hostDir, err := s.setupHostDir(false)
		if err != nil {
			return "", err
		}
		return s.runner(ctx, "git", append(append([]string{}, journeyGitFlags...), command...), hostDir)
	}
	gitDir := filepath.Join(dir, ".git")
	_, gitErr := os.Lstat(gitDir)
	if errors.Is(gitErr, os.ErrNotExist) && len(settings) == 0 && len(command) == 3 && command[0] == "config" && command[1] == "--get" && (command[2] == "user.name" || command[2] == "user.email") {
		if _, err := hostpath.ResolveOutside(root, dir); err != nil {
			return "", errors.New("Git identity must be read outside the projects folder")
		}
		return s.runner(ctx, "git", append(append([]string{}, journeyGitFlags...), command...), dir)
	}
	all := append(append([]string{}, journeyGitFlags...), settings...)
	if errors.Is(gitErr, os.ErrNotExist) && command[0] == "init" {
		// Restore initializes a controller-created empty directory in the
		// assigned root. Never let a missing .git fall back to parent discovery.
		if _, err := hostpath.Confine(root, dir); err != nil {
			return "", err
		}
		if _, err := hostpath.ConfineToMounts([]string{root}, dir); err != nil {
			return "", err
		}
		all = append(all, "--git-dir="+gitDir, "--work-tree="+dir, "-c", "init.templateDir=", "-c", "core.bare=false")
	} else {
		bounded, err := journeyGitBoundary(ctx, root, dir, command)
		if err != nil {
			return "", err
		}
		if selected, ok := ctx.Value(privateReviewGitKey{}).(privateReviewGit); ok {
			// Only the controller creates this directory. Confinement and
			// repository configuration are still validated against dir above.
			bounded[0] = "--git-dir=" + selected.GitDir
			bounded[1] = "--work-tree=" + selected.Worktree
			all = append(all, "-c", "core.splitIndex=false")
		}
		all = append(all, bounded[:len(bounded)-len(command)]...)
	}
	ctx = context.WithValue(ctx, managedJourneyGitContextKey{}, true)
	if command[0] != "config" && command[0] != "init" && !journeyGitMetadataOnly(command) {
		// Even `git status` may run a clean filter when it compares a tracked
		// file to the index. Neutralize configured drivers before every command
		// that can inspect the worktree, not only before staging a commit.
		probe := append(append([]string{}, all...), "config", "--null", "--name-only", "--get-regexp", `^filter\.`)
		filters, filterErr := s.runner(ctx, "git", probe, dir)
		if filterErr != nil {
			var exit *exec.ExitError
			if ctx.Err() != nil || strings.TrimSpace(filters) != "" || !errors.As(filterErr, &exit) || exit.ExitCode() != 1 {
				return "", filterErr
			}
		}
		seen := map[string]bool{}
		for _, key := range strings.Split(filters, "\x00") {
			if key == "" {
				continue
			}
			end := strings.LastIndexByte(key, '.')
			if !strings.HasPrefix(key, "filter.") || end <= len("filter.") || strings.ContainsAny(key, "\r\n") {
				return "", errors.New("Git filter configuration cannot be confined safely")
			}
			driver := key[:end]
			if seen[driver] {
				continue
			}
			seen[driver] = true
			for _, setting := range []string{"clean=", "smudge=", "process=", "required=false"} {
				all = append(all, "-c", driver+"."+setting)
			}
		}
	}
	all = append(all, command...)
	return s.runner(ctx, "git", all, dir)
}

func parseGitChanges(status string) []GitChange {
	changes := []GitChange{}
	parts := strings.Split(status, "\x00")
	for i := 0; i < len(parts); i++ {
		line := parts[i]
		if len(line) < 4 {
			continue
		}
		changes = append(changes, GitChange{Path: line[3:], Status: line[:2]})
		// Porcelain -z reports a rename/copy destination first, then its source.
		if strings.ContainsAny(line[:2], "RC") && i+1 < len(parts) {
			i++
		}
	}
	return changes
}

func (s *Server) gitSummary(ctx context.Context, dir string) (GitSummary, error) {
	result := GitSummary{Changes: []GitChange{}, Log: []GitLogEntry{}}
	review, err := s.captureGitReview(ctx, dir, "")
	if err != nil {
		return result, err
	}
	result.ReviewDigest = review.Digest
	branch, err := s.journeyGit(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		result.Branch = strings.TrimSpace(branch)
	} else {
		result.Branch = "detached"
	}
	status, err := s.journeyGit(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return result, err
	}
	result.Changes = parseGitChanges(status)
	result.Clean = len(result.Changes) == 0
	log, logErr := s.journeyGit(ctx, dir, "log", "-10", "--format=%H%x00%s%x00")
	if logErr == nil {
		parts := strings.Split(log, "\x00")
		for i := 0; i+1 < len(parts); i += 2 {
			hash := strings.TrimSpace(parts[i])
			if hash != "" {
				result.Log = append(result.Log, GitLogEntry{Hash: hash, Subject: parts[i+1]})
			}
		}
	}
	paths := []string{}
	for _, change := range result.Changes {
		if !blockedProjectFile(change.Path) || journeyContextFile(change.Path) {
			paths = append(paths, ":(literal)"+change.Path)
		}
	}
	if len(paths) == 0 {
		return result, nil
	}
	diffArgs := append([]string{"diff", "--no-ext-diff", "--no-textconv", "--"}, paths...)
	diff, err := s.journeyGit(ctx, dir, diffArgs...)
	if err != nil {
		return result, err
	}
	stagedArgs := append([]string{"diff", "--cached", "--no-ext-diff", "--no-textconv", "--"}, paths...)
	staged, err := s.journeyGit(ctx, dir, stagedArgs...)
	if err != nil {
		return result, err
	}
	result.Diff = diff + review.Preview
	if staged != "" {
		result.Diff += "\nStaged changes:\n" + staged
	}
	if len(result.Diff) > maxProjectFile {
		result.Diff = result.Diff[:maxProjectFile] + "\n[Diff truncated at 1 MiB]"
	}
	latest, err := s.captureGitReview(ctx, dir, "")
	if err != nil {
		return result, err
	}
	if latest.Digest != review.Digest {
		return result, errors.New("The source changed during review. Refresh to inspect the latest changes.")
	}
	return result, nil
}

func (s *Server) projectGit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx = context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir)
	result, err := s.gitSummary(ctx, p.Path)
	if err != nil {
		journeyError(w, 422, "git_failed", err.Error())
		return
	}
	journeyJSON(w, 200, result)
}

func (s *Server) commitProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message              string `json:"message"`
		ExpectedReviewDigest string `json:"expectedReviewDigest"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Message) == "" || len(req.Message) > 4000 || strings.ContainsRune(req.Message, '\x00') {
		journeyError(w, 400, "invalid_message", "Write a commit message of 1–4,000 characters.")
		return
	}
	if !hexDigest(req.ExpectedReviewDigest) {
		journeyError(w, 400, "review_required", "Refresh and inspect the Git review before committing. Its source identity is required.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	// Resolve identity outside this repository, so agent-edited local config
	// cannot silently impersonate another author. Explicit Luci values win.
	name, email := os.Getenv("LUCI_GIT_NAME"), os.Getenv("LUCI_GIT_EMAIL")
	if name == "" || email == "" {
		identityDir, err := os.MkdirTemp("", "luci-identity-*")
		if err != nil {
			journeyError(w, 500, "identity_unavailable", err.Error())
			return
		}
		defer os.Remove(identityDir)
		if name == "" {
			name, _ = s.journeyGit(ctx, identityDir, "config", "--get", "user.name")
		}
		if email == "" {
			email, _ = s.journeyGit(ctx, identityDir, "config", "--get", "user.email")
		}
	}
	name, email = strings.TrimSpace(name), strings.TrimSpace(email)
	if name == "" || email == "" || strings.ContainsAny(name+email, "\r\n\x00") {
		journeyError(w, 409, "git_identity_required", "Set your Git user.name and user.email, or LUCI_GIT_NAME and LUCI_GIT_EMAIL, before committing.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx = context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir)
	original, err := s.captureGitReview(ctx, p.Path, "")
	if err != nil {
		journeyError(w, 422, "git_failed", err.Error())
		return
	}
	if original.Digest != req.ExpectedReviewDigest {
		journeyError(w, 409, "source_changed", "The source, project context, branch or staging changed since this review. Refresh and inspect the latest changes; your commit message is kept.")
		return
	}
	if !strings.HasPrefix(original.Branch, "refs/heads/") {
		journeyError(w, 409, "branch_required", "Check out or create a named branch in Git, then refresh this review before committing.")
		return
	}
	status, err := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		journeyError(w, 422, "git_failed", err.Error())
		return
	}
	changes := parseGitChanges(status)
	if len(changes) == 0 {
		journeyError(w, 409, "nothing_to_commit", "The project has no changes to commit.")
		return
	}
	archiveFiles := map[string]bool{}
	for _, change := range changes {
		if change.Path == ".luci/state.json" || journeyArchivePath(change.Path) {
			state, err := readJourney(p.Path, p.ID)
			if err != nil {
				journeyError(w, 409, "context_unavailable", "Saved context must be readable before committing its history.")
				return
			}
			records, err := readJourneyArchive(p.Path, state.ContextID, state.HistoryHead)
			if err != nil {
				journeyError(w, 409, "archive_unavailable", err.Error())
				return
			}
			for _, record := range records {
				archiveFiles[journeyArchiveFile(record.Digest)] = true
			}
			break
		}
	}
	for _, change := range changes {
		if journeyArchivePath(change.Path) && !strings.Contains(change.Status, "D") && !archiveFiles[change.Path] {
			journeyError(w, 409, "unreferenced_archive", "A context archive file is outside the verified history chain. Export or remove it explicitly before committing.")
			return
		}
		// .luci is intentional provenance. Other credential names require a
		// user's explicit handling in Git; the one-click commit never adds them.
		if blockedProjectFile(change.Path) && !journeyContextFile(change.Path) {
			journeyError(w, 409, "sensitive_file", "Review the credential-like file in Git before committing: "+change.Path)
			return
		}
		if !strings.Contains(change.Status, "D") {
			if _, err := guardedProjectPath(p.Path, change.Path, false); err != nil {
				journeyError(w, 403, "unsafe_path", err.Error())
				return
			}
		}
	}
	// Standard Git lock files exclude concurrent staging and checkout. Keep
	// existing staging byte-for-byte until the exact reviewed commit publishes.
	indexPath := filepath.Join(original.GitDir, "index")
	indexLock, err := os.OpenFile(indexPath+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		journeyError(w, 409, "git_busy", "Git is changing this project. Wait for it to finish, then refresh the review.")
		return
	}
	indexLockOwned := true
	defer func() {
		indexLock.Close()
		if indexLockOwned {
			os.Remove(indexPath + ".lock")
		}
	}()
	headLock, err := os.OpenFile(filepath.Join(original.GitDir, "HEAD.lock"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		journeyError(w, 409, "git_busy", "Git is changing branches. Wait for it to finish, then refresh the review.")
		return
	}
	defer func() { headLock.Close(); os.Remove(filepath.Join(original.GitDir, "HEAD.lock")) }()
	temp, err := os.MkdirTemp("", "luci-reviewed-commit-*")
	if err != nil {
		journeyError(w, 500, "snapshot_failed", "The reviewed source could not be captured.")
		return
	}
	defer os.RemoveAll(temp)
	worktree := filepath.Join(temp, "worktree")
	if err := os.Mkdir(worktree, 0700); err != nil {
		journeyError(w, 500, "snapshot_failed", err.Error())
		return
	}
	captured, err := s.captureGitReview(ctx, p.Path, worktree)
	if err != nil {
		journeyError(w, 409, "source_changed", "The source could not be captured consistently. Refresh the Git review.")
		return
	}
	if captured.Digest != req.ExpectedReviewDigest {
		journeyError(w, 409, "source_changed", "The project changed since this review. Refresh and inspect the latest changes; your commit message is kept.")
		return
	}
	private, publication, err := preparePrivateReviewGit(temp, captured)
	if err != nil {
		journeyError(w, 500, "snapshot_failed", err.Error())
		return
	}
	privateCtx := context.WithValue(ctx, privateReviewGitKey{}, private)
	originalIndex, indexReadErr := readGitReviewFile(indexPath, 16<<20)
	if indexReadErr == nil {
		if len(originalIndex) > 16<<20 {
			journeyError(w, 413, "index_too_large", "The Git index exceeds the reviewed commit limit.")
			return
		}
		if err := os.WriteFile(filepath.Join(private.GitDir, "index"), originalIndex, 0600); err != nil {
			journeyError(w, 422, "stage_failed", err.Error())
			return
		}
	} else if errors.Is(indexReadErr, os.ErrNotExist) {
		if _, err := s.journeyGit(privateCtx, p.Path, "read-tree", "--empty"); err != nil {
			journeyError(w, 422, "stage_failed", err.Error())
			return
		}
	} else {
		journeyError(w, 422, "stage_failed", indexReadErr.Error())
		return
	}
	if captured.SharedIndexName != "" {
		if _, err := s.journeyGit(privateCtx, p.Path, "update-index", "--no-split-index"); err != nil {
			journeyError(w, 422, "stage_failed", "The shared index could not be normalized. Original staging is unchanged.")
			return
		}
	}
	if err := s.stageReviewedPaths(privateCtx, p.Path, captured.Paths); err != nil {
		journeyError(w, 422, "stage_failed", err.Error())
		return
	}
	tree, err := s.journeyGit(privateCtx, p.Path, "write-tree")
	if err != nil {
		journeyError(w, 422, "stage_failed", err.Error())
		return
	}
	args := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "-c", "user.useConfigOnly=true", "commit-tree", strings.TrimSpace(tree), "-m", req.Message}
	if captured.Head != "" {
		args = append(args, "-p", captured.Head)
	}
	commit, err := s.journeyGit(privateCtx, p.Path, args...)
	commit = strings.TrimSpace(commit)
	if err != nil || !deliverySHA.MatchString(commit) {
		journeyError(w, 422, "commit_failed", "Git could not create the reviewed commit. Your original staging is unchanged.")
		return
	}
	installHeadLog, cleanupHeadLog, err := s.prepareReviewedHeadLog(privateCtx, p.Path, captured, private, commit, name, email, req.Message)
	if err != nil {
		journeyError(w, 422, "reflog_unavailable", "Git recovery history could not be prepared. Original staging and branch are unchanged.")
		return
	}
	defer cleanupHeadLog()
	// Prepare the replacement index before the ref publication. A ref conflict
	// leaves the original index untouched and only unreachable Git objects.
	index, err := os.ReadFile(filepath.Join(private.GitDir, "index"))
	if err != nil {
		journeyError(w, 422, "stage_failed", err.Error())
		return
	}
	if _, err := indexLock.Write(index); err != nil {
		journeyError(w, 422, "stage_failed", err.Error())
		return
	}
	if err := indexLock.Sync(); err != nil {
		journeyError(w, 422, "stage_failed", err.Error())
		return
	}
	if err := indexLock.Close(); err != nil {
		journeyError(w, 422, "stage_failed", err.Error())
		return
	}
	expectedHead := captured.Head
	if expectedHead == "" {
		expectedHead = strings.Repeat("0", captured.hashLength())
	}
	if _, err := s.journeyGit(context.WithValue(ctx, privateReviewGitKey{}, publication), p.Path, "-c", "user.name="+name, "-c", "user.email="+email, "-c", "user.useConfigOnly=true", "update-ref", "--create-reflog", "-m", "commit: "+strings.Split(req.Message, "\n")[0], captured.Branch, commit, expectedHead); err != nil {
		journeyError(w, 409, "source_changed", "The branch changed before the commit could be published. Refresh the review. Original staging is unchanged.")
		return
	}
	if err := installHeadLog(); err != nil {
		journeyError(w, 500, "commit_reflog_failed", "The reviewed commit was created, but its HEAD recovery log could not be installed. Refresh Git before another commit.")
		return
	}
	if err := os.Rename(indexPath+".lock", indexPath); err != nil {
		journeyError(w, 500, "commit_index_failed", "The reviewed commit was created, but the Git index could not be updated. Refresh Git and recover its index before committing again.")
		return
	}
	indexLockOwned = false
	// Subsequent edits stay in the real worktree and are not swept into the
	// immutable commit. Report them without turning a successful commit into a
	// retryable error if this final observation is unavailable.
	status, statusErr := s.journeyGit(ctx, p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	journeyJSON(w, 200, map[string]any{"commit": commit, "message": "Reviewed changes committed.", "sourceChanged": statusErr != nil || status != ""})
}
