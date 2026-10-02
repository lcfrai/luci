package product

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
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
	Branch  string        `json:"branch"`
	Clean   bool          `json:"clean"`
	Changes []GitChange   `json:"changes"`
	Log     []GitLogEntry `json:"log"`
	Diff    string        `json:"diff"`
}

// These flags keep read-only Git operations from invoking repository hooks,
// fsmonitor commands, pagers or external diff/textconv programs.
var journeyGitFlags = []string{"--no-pager", "--no-optional-locks", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "color.ui=false", "-c", "commit.gpgsign=false"}

func (s *Server) journeyGit(ctx context.Context, dir string, args ...string) (string, error) {
	all := append([]string{}, journeyGitFlags...)
	if len(args) > 0 && args[0] != "config" {
		// Even `git status` may run a clean filter when it compares a tracked
		// file to the index. Neutralize configured drivers before every command
		// that can inspect the worktree, not only before staging a commit.
		probe := append(append([]string{}, journeyGitFlags...), "config", "--null", "--name-only", "--get-regexp", `^filter\.`)
		filters, filterErr := s.runner(ctx, "git", probe, dir)
		if filterErr != nil && strings.TrimSpace(filters) != "" {
			return "", filterErr
		}
		seen := map[string]bool{}
		for _, key := range strings.Split(filters, "\x00") {
			key = strings.TrimSpace(key)
			end := strings.LastIndexByte(key, '.')
			if !strings.HasPrefix(key, "filter.") || end <= len("filter.") {
				continue
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
	all = append(all, args...)
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
	branch, err := s.journeyGit(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		result.Branch = strings.TrimSpace(branch)
	} else {
		result.Branch = "detached"
	}
	status, err := s.journeyGit(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=normal")
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
		if !blockedProjectFile(change.Path) {
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
	result.Diff = diff
	if staged != "" {
		result.Diff += "\nStaged changes:\n" + staged
	}
	if len(result.Diff) > maxProjectFile {
		result.Diff = result.Diff[:maxProjectFile] + "\n[Diff truncated at 1 MiB]"
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
	result, err := s.gitSummary(ctx, p.Path)
	if err != nil {
		journeyError(w, 422, "git_failed", err.Error())
		return
	}
	journeyJSON(w, 200, result)
}

func (s *Server) commitProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Message) == "" || len(req.Message) > 4000 || strings.ContainsRune(req.Message, '\x00') {
		journeyError(w, 400, "invalid_message", "Write a commit message of 1–4,000 characters.")
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
	for _, change := range changes {
		// .luci is intentional provenance. Other credential names require a
		// user's explicit handling in Git; the one-click commit never adds them.
		if blockedProjectFile(change.Path) && change.Path != ".luci/state.json" {
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
	if _, err := s.journeyGit(ctx, p.Path, "add", "--all", "--", "."); err != nil {
		journeyError(w, 422, "stage_failed", err.Error())
		return
	}
	args := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "-c", "user.useConfigOnly=true", "commit", "--no-gpg-sign", "-m", req.Message}
	output, err := s.journeyGit(ctx, p.Path, args...)
	if err != nil {
		journeyError(w, 422, "commit_failed", fmt.Sprintf("Changes remain staged. Git reported: %v", err))
		return
	}
	hash, err := s.journeyGit(ctx, p.Path, "rev-parse", "HEAD")
	if err != nil {
		journeyError(w, 500, "commit_verification_failed", "Git committed but its resulting revision could not be read: "+err.Error())
		return
	}
	journeyJSON(w, 200, map[string]string{"commit": strings.TrimSpace(hash), "message": strings.TrimSpace(output)})
}
