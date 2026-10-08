package product

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

const swarmCheckpointFileLimit = 128 << 10
const swarmCheckpointTotalLimit = 512 << 10
const swarmCheckpointPathLimit = 128

type checkpointTreeFile struct {
	Mode, Hash string
	Size       int64
}
type checkpointCapture struct {
	Review                     ProjectSwarmCheckpointReview
	SourceDigest, TargetDigest string
}

func (s *Server) reviewProjectSwarmCheckpoint(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		projectRunProblem(w, ErrProjectRunInvalid)
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	op := swarmOperation(&run, r.PathValue("operationId"))
	if op == nil {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	capture, err := s.captureSwarmCheckpoint(ctx, p, run, *op)
	if err != nil {
		// A refusal never returns a partial candidate that could be mistaken
		// for the complete continuation review.
		capture.Review = ProjectSwarmCheckpointReview{ProjectID: p.ID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, TaskID: op.TaskID, ExpectedRevision: run.Revision, Files: []ProjectSwarmCheckpointFile{}, Reason: err.Error()}
	}
	journeyJSON(w, 200, map[string]any{"review": capture.Review})
}

func (s *Server) checkpointTree(ctx context.Context, dir, commit string) (map[string]checkpointTreeFile, error) {
	out, err := s.swarmGit(ctx, dir, "ls-tree", "-r", "-z", "-l", "--full-tree", commit)
	if err != nil {
		return nil, errors.New("The checkpoint base tree could not be read.")
	}
	if len(out) >= 2<<20 {
		return nil, errors.New("The checkpoint source tree exceeds the complete review limit.")
	}
	result := map[string]checkpointTreeFile{}
	for _, row := range strings.Split(out, "\x00") {
		if row == "" {
			continue
		}
		metadata, name, ok := strings.Cut(row, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 4 || !deliverySHA.MatchString(fields[2]) || len(result) >= 10000 {
			return nil, ErrProjectRunInvalid
		}
		size, e := strconv.ParseInt(fields[3], 10, 64)
		if e != nil || size < 0 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return nil, errors.New("Checkpoint continuation requires ordinary source files; linked files and submodules are unsupported.")
		}
		result[name] = checkpointTreeFile{fields[0], fields[2], size}
	}
	return result, nil
}

func (s *Server) openCheckpointRoot(p project.Project, run ProjectRun, op ProjectRunOperation) (*os.Root, error) {
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil || project.WorktreeID(path) != op.WorktreeID {
		return nil, ErrProjectRunInvalid
	}
	rootPath, err := guardedProjectPath(s.projectRoot(), "", false)
	if err != nil {
		return nil, errors.New("The project storage is unavailable for safe checkpoint inspection.")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, errors.New("The project storage could not be opened for safe checkpoint inspection.")
	}
	for _, part := range []string{".luci-swarms", p.ID, run.ID, op.ID} {
		next, e := openSwarmChildRoot(root, part)
		root.Close()
		if e != nil {
			return nil, errors.New("The retained workspace is missing or contains an unsafe directory link.")
		}
		root = next
	}
	return root, nil
}

// Hash the actual opened file independently of worker index flags and Git
// filters. Large unchanged files need no displayed content; a changed file
// outside the byte budget refuses the entire candidate.
func readCheckpointText(root *os.Root, name, format, indexMode string, prior checkpointTreeFile, total *int64) (string, string, string, error) {
	f, err := openSwarmWorkspaceFile(root, name)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", "", nil
	}
	if err != nil {
		return "", "", "", errors.New("A checkpoint path is linked, ambiguous, or not an ordinary file.")
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || before.Size() > (100<<20)-*total {
		return "", "", "", errors.New("The checkpoint source exceeds its complete read limit.")
	}
	*total += before.Size()
	var h hash.Hash = sha1.New()
	if format == "sha256" {
		h = sha256.New()
	}
	fmt.Fprintf(h, "blob %d\x00", before.Size())
	buffer := &limitedBuffer{limit: swarmCheckpointFileLimit + 1}
	n, err := io.Copy(io.MultiWriter(h, buffer), io.LimitReader(f, before.Size()+1))
	after, statErr := f.Stat()
	if err != nil || statErr != nil || n != before.Size() || after.Size() != before.Size() || !before.ModTime().Equal(after.ModTime()) || swarmWorkspaceSingleLink(f) != nil {
		return "", "", "", errors.New("The checkpoint source changed while reading. Refresh its review.")
	}
	mode := "100644"
	if before.Mode().Perm()&0111 != 0 {
		mode = "100755"
	}
	if runtime.GOOS == "windows" {
		if indexMode == "100644" || indexMode == "100755" {
			mode = indexMode
		} else if prior.Mode != "" {
			mode = prior.Mode
		}
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if digest == prior.Hash && mode == prior.Mode {
		return "", digest, mode, nil
	}
	data := buffer.String()
	if n > swarmCheckpointFileLimit || !utf8.ValidString(data) || strings.ContainsRune(data, '\x00') {
		return "", "", "", errors.New("Checkpoint continuation currently requires complete UTF-8 text changes of at most 128 KiB per file. Binary and larger output remains retained for inspection.")
	}
	return data, digest, mode, nil
}

func (s *Server) captureSwarmCheckpoint(ctx context.Context, p project.Project, run ProjectRun, op ProjectRunOperation) (checkpointCapture, error) {
	var result checkpointCapture
	if err := eligibleSwarmCheckpoint(run, &op); err != nil {
		return result, err
	}
	if s.projectSwarmCheckpointGuard == nil || s.projectSwarmCheckpointAuthorize == nil {
		return result, errors.New("This execution service cannot yet prove that the original attempt has stopped safely.")
	}
	active, err := s.swarmWorktreeActive(ctx, p.ID, op.WorktreeID)
	if err != nil || active {
		return result, errors.New("The original worker is still active or its state is uncertain. Finish its recovery before continuing output.")
	}
	if err = s.OrchestratorExecutionGate(p.ID); err != nil {
		return result, err
	}
	if err = s.validateProjectRunApprovals(p.ID, run.Input.Approvals); err != nil {
		return result, err
	}
	s.mu.RLock()
	state, stateErr := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if freshness := projectRunContextFreshness(run, state, stateErr); freshness.Status != "current" {
		return result, errors.New(freshness.Reason)
	}
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		return result, errors.New("The retained workspace ownership is unavailable or unsafe.")
	}
	if err = s.validateSwarmTaskGit(ctx, path); err != nil {
		return result, errors.New("The retained workspace has unsafe or unavailable Git metadata. Inspect its source before continuing.")
	}
	if err = s.validateSwarmGit(ctx, p.Path); err != nil {
		return result, errors.New("The primary project has unsafe or unavailable Git metadata. Review it before continuing.")
	}
	source, err := s.captureGitReview(ctx, path, "")
	if err != nil {
		return result, errors.New("The retained source cannot be completely reviewed. Remove unsafe Git flags or metadata and inspect its files before trying again.")
	}
	target, err := s.captureGitReview(ctx, p.Path, "")
	if err != nil || target.Status != "" || target.Head != swarmExpectedHead(run) {
		return result, errors.New("The main project changed outside this build's reviewed integrations or has pending edits. Review the current source before continuing.")
	}
	if !deliverySHA.MatchString(source.Head) {
		return result, ErrProjectRunInvalid
	}
	if _, err = s.swarmGit(ctx, path, "merge-base", "--is-ancestor", op.BaseCommit, source.Head); err != nil {
		return result, errors.New("The retained task history no longer descends from its assigned base.")
	}
	base, err := s.checkpointTree(ctx, path, op.BaseCommit)
	if err != nil {
		return result, err
	}
	current, err := s.checkpointTree(ctx, p.Path, target.Head)
	if err != nil {
		return result, err
	}
	changed, err := s.swarmGit(ctx, path, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", op.BaseCommit, "--")
	if err != nil || len(changed) >= 2<<20 {
		return result, errors.New("The complete retained change list is unavailable.")
	}
	for _, name := range strings.Split(changed, "\x00") {
		if name != "" && (!safeSwarmWorkspacePath(name) || strings.EqualFold(name, "LUCI.md")) {
			return result, errors.New("This attempt changes protected project context or an unsupported path. Use the reviewed context amendment workflow before continuing.")
		}
	}
	root, err := s.openCheckpointRoot(p, run, op)
	if err != nil {
		return result, err
	}
	defer root.Close()
	paths := map[string]bool{}
	for name := range base {
		paths[name] = true
	}
	for _, name := range source.Paths {
		paths[name] = true
	}
	// Includes indexed new files even if the worker changed ignore rules.
	listed, err := s.swarmGit(ctx, path, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil || len(listed) >= 2<<20 {
		return result, errors.New("The complete retained file list is unavailable.")
	}
	for _, name := range strings.Split(listed, "\x00") {
		if name != "" {
			paths[name] = true
		}
	}
	if len(paths) > 10000 {
		return result, errors.New("The checkpoint source exceeds 10,000 files.")
	}
	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)
	result.Review = ProjectSwarmCheckpointReview{ProjectID: p.ID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, TaskID: op.TaskID, ExpectedRevision: run.Revision, SourceCommit: source.Head, TargetCommit: target.Head, Files: []ProjectSwarmCheckpointFile{}}
	task := swarmTask(&run, op.TaskID)
	indexModes := map[string]string{}
	for _, row := range strings.Split(source.Index, "\x00") {
		metadata, name, ok := strings.Cut(row, "\t")
		fields := strings.Fields(metadata)
		if ok && len(fields) == 3 && fields[2] == "0" {
			indexModes[name] = fields[0]
		}
	}
	var totalRead int64
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		prior := base[name]
		if !safeSwarmWorkspacePath(name) || strings.EqualFold(name, "LUCI.md") {
			if prior.Hash == "" {
				return result, errors.New("New protected or unsupported files cannot be adopted into a checkpoint.")
			}
			continue // Never adopt canonical context or private files from a worker.
		}
		after, digest, mode, e := readCheckpointText(root, name, source.ObjectFormat, indexModes[name], prior, &totalRead)
		if e != nil {
			return result, e
		}
		if digest == prior.Hash && mode == prior.Mode {
			continue
		}
		if len(task.WritePaths) > 0 && !swarmPathAllowed(name, task.WritePaths) {
			return result, errors.New("Retained changes extend outside this task's reviewed write scope. Amend the plan before continuing.")
		}
		if current[name] != prior {
			return result, errors.New("The main project also changed a retained file. Review that overlap explicitly; Luci will not overwrite or automatically rebase it.")
		}
		file := ProjectSwarmCheckpointFile{Path: name, After: after, BeforeMode: prior.Mode, AfterMode: mode, Status: "modified"}
		if prior.Hash == "" {
			file.Status = "added"
		} else {
			if prior.Size > swarmCheckpointFileLimit {
				return result, errors.New("A changed file's original content exceeds the complete 128 KiB review limit.")
			}
			file.Before, e = s.swarmGit(ctx, path, "cat-file", "blob", prior.Hash)
			if e != nil || int64(len(file.Before)) != prior.Size || !utf8.ValidString(file.Before) || strings.ContainsRune(file.Before, '\x00') {
				return result, errors.New("A changed file has binary or unavailable original content; no partial checkpoint is accepted.")
			}
		}
		if mode == "" {
			file.Status = "deleted"
		}
		result.Review.TotalBytes += len(file.Before) + len(file.After)
		if result.Review.TotalBytes > swarmCheckpointTotalLimit || len(result.Review.Files) >= swarmCheckpointPathLimit {
			return result, errors.New("The checkpoint exceeds the complete review limit of 128 files or 512 KiB. Split the work before continuing.")
		}
		result.Review.Files = append(result.Review.Files, file)
	}
	if len(result.Review.Files) == 0 {
		return result, errors.New("This attempt has no retained source changes to carry forward. Retry from the current project instead.")
	}
	if err := s.validateCheckpointTargetPaths(ctx, p.Path, current, result.Review.Files); err != nil {
		return result, err
	}
	end, err := s.captureGitReview(ctx, path, "")
	if err != nil || end.Digest != source.Digest {
		return result, errors.New("The retained source changed while its checkpoint was being read. Refresh the complete review.")
	}
	endTarget, err := s.captureGitReview(ctx, p.Path, "")
	if err != nil || endTarget.Digest != target.Digest {
		return result, errors.New("The main project changed during checkpoint review. Refresh before continuing.")
	}
	result.SourceDigest, result.TargetDigest = source.Digest, target.Digest
	result.Review.ReviewDigest = implementationDigest(struct {
		Source, Target, Base, Context string
		Review                        ProjectSwarmCheckpointReview
	}{source.Digest, target.Digest, op.BaseCommit, implementationDigest(op.Implementation), result.Review})
	result.Review.CanContinue = true
	result.Review.Reason = "Review every retained change before carrying it into a new isolated attempt. Ignored files are not copied. The task stays incomplete and starting an agent is a separate action."
	return result, nil
}

func (s *Server) validateCheckpointTargetPaths(ctx context.Context, dir string, target map[string]checkpointTreeFile, files []ProjectSwarmCheckpointFile) error {
	seen := map[string]string{}
	for _, file := range files {
		if strings.EqualFold(filepath.Base(file.Path), ".gitattributes") {
			return errors.New("Changed Git conversion rules require a separate source review before checkpoint continuation.")
		}
		fold := strings.ToUpper(file.Path)
		if previous, ok := seen[fold]; ok && previous != file.Path {
			return errors.New("Checkpoint paths must remain distinct on Windows. Rename ambiguous files before continuing.")
		}
		seen[fold] = file.Path
		for name := range target {
			other := strings.ToUpper(name)
			if other == fold && name != file.Path || file.AfterMode != "" && (strings.HasPrefix(other, fold+"/") || strings.HasPrefix(fold, other+"/")) {
				return errors.New("A checkpoint path conflicts with an existing file, directory or Windows spelling. Review that source change before continuing.")
			}
		}
		// A raw-text checkpoint must not silently reinterpret its reviewed bytes
		// through a project filter, encoding or newline conversion on checkout.
		attrs, err := s.swarmGit(ctx, dir, "check-attr", "-z", "--all", "--", file.Path)
		if err != nil || len(attrs) >= 2<<20 {
			return errors.New("The checkpoint's source conversion rules could not be reviewed.")
		}
		parts := strings.Split(strings.TrimSuffix(attrs, "\x00"), "\x00")
		if attrs == "" {
			continue
		}
		if len(parts)%3 != 0 {
			return ErrProjectRunInvalid
		}
		for i := 0; i < len(parts); i += 3 {
			attribute, value := parts[i+1], parts[i+2]
			if value == "unset" || value == "unspecified" {
				continue
			}
			if attribute == "filter" || attribute == "working-tree-encoding" || attribute == "ident" || ((attribute == "text" || attribute == "eol") && strings.ContainsRune(file.After, '\r')) {
				return errors.New("A changed file uses source conversion rules that cannot preserve these exact reviewed text bytes. Preserve the output and resolve its encoding or filter before continuing.")
			}
		}
	}
	return nil
}
