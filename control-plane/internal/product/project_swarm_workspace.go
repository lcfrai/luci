package product

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

const swarmWorkspaceFileLimit = 128 << 10
const swarmWorkspaceListLimit = 2000
const swarmWorkspaceWalkLimit = 10000

type ProjectSwarmWorkspace struct {
	OperationID    string      `json:"operationId"`
	TaskID         string      `json:"taskId"`
	WorktreeID     string      `json:"worktreeId"`
	Exists         bool        `json:"exists"`
	Files          []GitChange `json:"files"`
	FilesOmitted   int         `json:"filesOmitted"`
	FilesTruncated bool        `json:"filesTruncated"`
	Patch          string      `json:"patch"`
	PatchTruncated bool        `json:"patchTruncated"`
	Reason         string      `json:"reason"`
}

type ProjectSwarmWorkspaceFile struct {
	OperationID string `json:"operationId"`
	TaskID      string `json:"taskId"`
	WorktreeID  string `json:"worktreeId"`
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	Content     string `json:"content"`
	Binary      bool   `json:"binary"`
	Omitted     bool   `json:"omitted"`
	Reason      string `json:"reason"`
}

// Retained output is identified by the private reservation, even when allocation
// never created a session. It is a current filesystem observation, not approval,
// immutable integration review, or permission to restart the previous attempt.
func (s *Server) ownedSwarmWorkspace(w http.ResponseWriter, r *http.Request) (ProjectRunOperation, string, *os.Root, bool) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return ProjectRunOperation{}, "", nil, false
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return ProjectRunOperation{}, "", nil, false
	}
	op := swarmOperation(&run, r.PathValue("operationId"))
	if run.Swarm == nil || op == nil || op.TaskID == "" || swarmTask(&run, op.TaskID) == nil {
		journeyError(w, 404, "swarm_workspace_unavailable", "This task attempt does not belong to the selected build.")
		return ProjectRunOperation{}, "", nil, false
	}
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil || op.WorktreeID != project.WorktreeID(path) {
		journeyError(w, 409, "swarm_workspace_unavailable", "The retained workspace ownership could not be verified. No files were read or changed.")
		return ProjectRunOperation{}, "", nil, false
	}
	rootPath, err := guardedProjectPath(s.projectRoot(), "", false)
	if err != nil {
		journeyError(w, 409, "swarm_workspace_unavailable", "The project storage is unavailable for safe inspection.")
		return ProjectRunOperation{}, "", nil, false
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		journeyError(w, 409, "swarm_workspace_unavailable", "The project storage could not be opened for inspection.")
		return ProjectRunOperation{}, "", nil, false
	}
	for _, part := range []string{".luci-swarms", p.ID, run.ID, op.ID} {
		next, openErr := openSwarmChildRoot(root, part)
		root.Close()
		if errors.Is(openErr, os.ErrNotExist) {
			return *op, path, nil, true
		}
		if openErr != nil {
			journeyError(w, 409, "swarm_workspace_unavailable", "The retained workspace changed or contains an unsafe directory link. No files were read or changed.")
			return ProjectRunOperation{}, "", nil, false
		}
		root = next
	}
	return *op, path, root, true
}

func openSwarmChildRoot(parent *os.Root, name string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return nil, errors.New("not an ordinary directory")
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		child.Close()
		return nil, errors.New("directory changed")
	}
	return child, nil
}

func safeSwarmWorkspacePath(path string) bool {
	if !validProjectTaskWritePath(path) || blockedProjectFile(path) || blockedProjectFile(strings.ToUpper(path)) {
		return false
	}
	for _, component := range strings.Split(path, "/") {
		// Windows may resolve an 8.3 alias to a protected long filename.
		// Refuse that ambiguous spelling on every host; use the full name.
		if index := strings.LastIndexByte(component, '~'); index >= 0 {
			suffix := strings.SplitN(component[index+1:], ".", 2)[0]
			digits := suffix != ""
			for _, digit := range suffix {
				if digit < '0' || digit > '9' {
					digits = false
					break
				}
			}
			if digits {
				return false
			}
		}
	}
	return true
}

func (s *Server) inspectProjectSwarmWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		journeyError(w, 400, "invalid_swarm_workspace_request", "Inspect this task workspace without additional parameters.")
		return
	}
	op, path, root, ok := s.ownedSwarmWorkspace(w, r)
	if !ok {
		return
	}
	out := ProjectSwarmWorkspace{OperationID: op.ID, TaskID: op.TaskID, WorktreeID: op.WorktreeID, Files: []GitChange{}}
	if root == nil {
		out.Reason = "No retained workspace exists for this reservation. No agent session or files were created by this inspection."
		journeyJSON(w, 200, map[string]any{"workspace": out})
		return
	}
	defer root.Close()
	out.Exists = true
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	visited := 0
	var walk func(*os.Root, string, int) error
	walk = func(dir *os.Root, prefix string, depth int) error {
		if depth > 64 {
			out.FilesTruncated = true
			out.FilesOmitted++
			return nil
		}
		file, err := dir.Open(".")
		if err != nil {
			return err
		}
		defer file.Close()
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			entries, readErr := file.ReadDir(128)
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return readErr
			}
			for _, item := range entries {
				visited++
				if visited > swarmWorkspaceWalkLimit || len(out.Files) >= swarmWorkspaceListLimit {
					out.FilesTruncated = true
					out.FilesOmitted++
					return nil
				}
				relative := item.Name()
				if prefix != "" {
					relative = prefix + "/" + relative
				}
				if !safeSwarmWorkspacePath(relative) {
					continue
				}
				info, err := dir.Lstat(item.Name())
				if err != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
					continue
				}
				if info.IsDir() {
					child, err := openSwarmChildRoot(dir, item.Name())
					if err != nil {
						continue
					}
					err = walk(child, relative, depth+1)
					child.Close()
					if err != nil || out.FilesTruncated {
						return err
					}
				} else if info.Mode().IsRegular() {
					opened, err := openSwarmRegularFile(dir, item.Name())
					if err != nil {
						continue
					}
					opened.Close()
					out.Files = append(out.Files, GitChange{Path: relative, Status: "saved"})
				}
			}
			if errors.Is(readErr, io.EOF) {
				return nil
			}
		}
	}
	if err := walk(root, "", 0); err != nil {
		journeyError(w, 409, "swarm_workspace_changed", "The retained files changed or could not be read completely. Refresh the inspection; no files were changed.")
		return
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	out.Reason = "Current retained output, including untracked files. Protected files, links, special files and ambiguous Windows aliases are excluded. Use full filenames. This inspection does not verify or integrate changes."
	// Git is optional: a failed allocation can leave ordinary output without a
	// complete repository. Never repair metadata or hide readable output then.
	if gitInfo, err := root.Lstat(".git"); err == nil && gitInfo.IsDir() && gitInfo.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 && s.validateSwarmGit(ctx, path) == nil {
		status, err := s.swarmGit(ctx, path, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=all")
		if err == nil {
			changes := map[string]string{}
			for _, change := range parseGitChanges(status) {
				if safeSwarmWorkspacePath(change.Path) {
					changes[change.Path] = change.Status
				}
			}
			if deliverySHA.MatchString(op.BaseCommit) {
				if names, err := s.swarmGit(ctx, path, "diff", "--no-renames", "--name-only", "-z", "--ignore-submodules=all", op.BaseCommit, "--"); err == nil {
					for _, name := range strings.Split(names, "\x00") {
						if safeSwarmWorkspacePath(name) && changes[name] == "" {
							changes[name] = "changed"
						}
					}
				}
			}
			// Prioritize actual changed/untracked output over large dependency
			// directories. Git only supplies candidate names: every file must
			// still pass the same pinned-directory and ordinary-file checks.
			changedPaths := make([]string, 0, len(changes))
			for name := range changes {
				changedPaths = append(changedPaths, name)
			}
			sort.Strings(changedPaths)
			prioritized := []GitChange{}
			seen := map[string]bool{}
			for _, name := range changedPaths {
				if ctx.Err() != nil {
					out.FilesTruncated = true
					break
				}
				if len(prioritized) == swarmWorkspaceListLimit {
					out.FilesTruncated = true
					out.FilesOmitted++
					break
				}
				file, err := openSwarmWorkspaceFile(root, name)
				if err != nil {
					continue
				}
				file.Close()
				prioritized = append(prioritized, GitChange{Path: name, Status: changes[name]})
				seen[name] = true
			}
			for _, saved := range out.Files {
				if seen[saved.Path] {
					continue
				}
				if len(prioritized) == swarmWorkspaceListLimit {
					out.FilesTruncated = true
					out.FilesOmitted++
					continue
				}
				prioritized = append(prioritized, saved)
			}
			out.Files = prioritized
			paths := []string{}
			pathBytes := 0
			for i := range out.Files {
				if value, ok := changes[out.Files[i].Path]; ok {
					out.Files[i].Status = value
				}
				// Leave room below Windows' command-line ceiling even for long
				// owned roots. Individual file reads remain available for every
				// listed path; this is only a bounded inspection diff.
				literal := ":(literal)" + out.Files[i].Path
				if len(paths) >= 128 || pathBytes+len(literal)+1 > 8<<10 {
					out.PatchTruncated = true
					continue
				}
				paths = append(paths, literal)
				pathBytes += len(literal) + 1
			}
			// Paths are literal and prefiltered; never request a repository-wide
			// diff that could contain credential files or private Luci documents.
			if len(paths) > 0 && deliverySHA.MatchString(op.BaseCommit) {
				args := append([]string{"diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--ignore-submodules=all", op.BaseCommit, "--"}, paths...)
				patch, err := s.swarmGit(ctx, path, args...)
				if err == nil {
					var truncated bool
					out.Patch, truncated = swarmBoundedText(patch, swarmWorkspaceFileLimit)
					out.PatchTruncated = out.PatchTruncated || truncated
				} else {
					out.PatchTruncated = true
				}
			}
			out.Reason += " The diff covers listed tracked files against the assigned base; open individual files for untracked output. Deleted or omitted files are not included in this inspection diff."
		} else {
			out.Reason += " Git status is unavailable; saved files remain readable."
		}
	} else {
		out.Reason += " No safe complete Git metadata is available; saved files remain readable."
	}
	if out.FilesTruncated {
		out.Reason += " The file inventory is incomplete. More output may exist beyond the inspection limit."
	}
	if out.PatchTruncated {
		out.Reason += " The diff is incomplete or unavailable; it is not an integration review."
	}
	journeyJSON(w, 200, map[string]any{"workspace": out})
}

func openSwarmRegularFile(root *os.Root, name string) (*os.File, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("not an ordinary file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || !after.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("file changed")
	}
	if err := swarmWorkspaceSingleLink(file); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func openSwarmWorkspaceFile(root *os.Root, relative string) (*os.File, error) {
	if !safeSwarmWorkspacePath(relative) {
		return nil, errors.New("unsafe retained path")
	}
	current, owned := root, false
	defer func() {
		if owned {
			current.Close()
		}
	}()
	parts := strings.Split(relative, "/")
	for _, part := range parts[:len(parts)-1] {
		next, err := openSwarmChildRoot(current, part)
		if err != nil {
			return nil, err
		}
		if owned {
			current.Close()
		}
		current, owned = next, true
	}
	return openSwarmRegularFile(current, parts[len(parts)-1])
}

func swarmBoundedText(value string, limit int) (string, bool) {
	value = strings.ToValidUTF8(value, "?")
	if len(value) <= limit {
		return value, false
	}
	value = value[:limit]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value, true
}

func (s *Server) readProjectSwarmWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) != 1 || len(query["path"]) != 1 {
		journeyError(w, 400, "invalid_swarm_file_request", "Choose one retained file using its repository-relative path.")
		return
	}
	relative := query.Get("path")
	if !safeSwarmWorkspacePath(relative) {
		journeyError(w, 403, "protected_swarm_file", "Only ordinary repository-relative files can be inspected. Credential files, Git metadata, private Luci context, links and ambiguous paths are excluded. Use the full filename rather than a Windows short-name alias.")
		return
	}
	op, _, root, ok := s.ownedSwarmWorkspace(w, r)
	if !ok {
		return
	}
	if root == nil {
		journeyError(w, 404, "swarm_workspace_missing", "This attempt has no retained workspace.")
		return
	}
	defer root.Close()
	file, err := openSwarmWorkspaceFile(root, relative)
	if err != nil {
		journeyError(w, 404, "swarm_file_unavailable", "The retained file is missing, linked, or is not ordinary saved output.")
		return
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		journeyError(w, 409, "swarm_file_changed", "The file changed before it could be inspected. Refresh the inspection.")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, swarmWorkspaceFileLimit+1))
	after, statErr := file.Stat()
	if err != nil || statErr != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || swarmWorkspaceSingleLink(file) != nil {
		journeyError(w, 409, "swarm_file_changed", "The file changed while it was being read. Refresh the inspection; no content was changed.")
		return
	}
	out := ProjectSwarmWorkspaceFile{OperationID: op.ID, TaskID: op.TaskID, WorktreeID: op.WorktreeID, Path: relative, Bytes: before.Size(), Omitted: len(data) > swarmWorkspaceFileLimit}
	if out.Omitted {
		data = data[:swarmWorkspaceFileLimit]
		for i := 0; i < 3 && len(data) > 0 && !utf8.Valid(data); i++ {
			data = data[:len(data)-1]
		}
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		out.Binary = true
		out.Omitted = true
		out.Reason = "This file is binary or is not UTF-8 text. Its bytes are retained and are not displayed."
	} else {
		out.Content = string(data)
		out.Reason = "Current retained file contents. This read does not verify, change, or integrate the file."
		if out.Omitted {
			out.Reason += " Only the first 128 KiB of text is displayed."
		}
	}
	journeyJSON(w, 200, map[string]any{"file": out})
}
