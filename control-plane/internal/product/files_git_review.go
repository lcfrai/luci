package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// A review covers Git-visible files (including portable context), their modes,
// index entries, branch and parent. The same bounded capture supplies the
// private commit worktree; validation never precedes an add of the live tree.
type gitReview struct {
	Digest, Branch, Head, Status, Index, GitDir, CommonDir, Preview, ObjectFormat string
	Paths                                                                         []string
	Metadata                                                                      map[string][]byte
	SharedIndexName                                                               string
	SharedIndex                                                                   []byte
}

type privateReviewGit struct{ GitDir, Worktree, ObjectsDir string }
type privateReviewGitKey struct{}

func (s *Server) captureGitReview(ctx context.Context, dir, destination string) (gitReview, error) {
	var result gitReview
	bounded, err := journeyGitBoundary(ctx, s.journeyGitRoot(ctx), dir, []string{"status"})
	if err != nil {
		return result, err
	}
	result.GitDir = strings.TrimPrefix(bounded[0], "--git-dir=")
	result.CommonDir = result.GitDir
	if common, err := os.ReadFile(filepath.Join(result.GitDir, "commondir")); err == nil {
		result.CommonDir = strings.TrimSpace(string(common))
		if !filepath.IsAbs(result.CommonDir) {
			result.CommonDir = filepath.Clean(filepath.Join(result.GitDir, result.CommonDir))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	branch, err := s.journeyGit(ctx, dir, "symbolic-ref", "--quiet", "HEAD")
	if err == nil {
		result.Branch = strings.TrimSpace(branch)
	} else if ctx.Err() != nil {
		return result, ctx.Err()
	}
	head, headErr := s.journeyGit(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if headErr == nil {
		result.Head = strings.TrimSpace(head)
	} else if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if result.Head != "" && !deliverySHA.MatchString(result.Head) {
		return result, errors.New("The Git parent could not be verified.")
	}
	result.Status, err = s.journeyGit(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return result, err
	}
	result.Index, err = s.journeyGit(ctx, dir, "ls-files", "--stage", "-z")
	if err != nil {
		return result, err
	}
	flags, err := s.journeyGit(ctx, dir, "ls-files", "-v", "-z")
	if err != nil {
		return result, err
	}
	if len(flags) >= 2<<20 {
		return result, errors.New("The Git review exceeds its metadata limit.")
	}
	for _, entry := range strings.Split(flags, "\x00") {
		if len(entry) > 0 && (entry[0] == 'S' || (entry[0] >= 'a' && entry[0] <= 'z')) {
			return result, errors.New("Clear Git skip-worktree and assume-unchanged flags (including sparse checkout), then refresh this review before committing in Luci.")
		}
	}
	shared, err := s.journeyGit(ctx, dir, "rev-parse", "--shared-index-path")
	if err != nil {
		return result, err
	}
	if shared = strings.TrimSpace(shared); shared != "" {
		if !filepath.IsAbs(shared) {
			shared = filepath.Join(dir, shared)
		}
		name := filepath.Base(shared)
		if filepath.Clean(filepath.Dir(shared)) != filepath.Clean(result.GitDir) || !strings.HasPrefix(name, "sharedindex.") || !deliverySHA.MatchString(strings.TrimPrefix(name, "sharedindex.")) {
			return result, errors.New("The shared Git index could not be confined.")
		}
		// With core.splitIndex=true Git can report an all-zero in-memory
		// placeholder even when the on-disk index is self-contained.
		if strings.Trim(strings.TrimPrefix(name, "sharedindex."), "0") != "" {
			result.SharedIndexName = name
			result.SharedIndex, err = readGitReviewFile(shared, 16<<20)
			if err != nil {
				return result, err
			}
		}
	}
	files, err := s.journeyGit(ctx, dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return result, err
	}
	for _, output := range []string{result.Status, result.Index, files} {
		if len(output) >= 2<<20 {
			return result, errors.New("The Git review exceeds its metadata limit.")
		}
	}
	format, err := s.journeyGit(ctx, dir, "rev-parse", "--show-object-format")
	if err != nil {
		return result, err
	}
	result.ObjectFormat = strings.TrimSpace(format)
	if result.ObjectFormat != "sha1" && result.ObjectFormat != "sha256" {
		return result, errors.New("This Git object format is unsupported for reviewed commits.")
	}
	result.Metadata = map[string][]byte{}
	h := sha256.New()
	fmt.Fprintf(h, "luci-git-review-v1\x00%s\x00%s\x00%s\x00%s\x00", result.Branch, result.Head, result.Status, result.Index)
	fmt.Fprintf(h, "format:%s\x00project:%s\x00", result.ObjectFormat, dir)
	// Configuration and attributes affect how Git interprets the reviewed bytes.
	for _, name := range []string{"config", "config.worktree", "info/attributes", "info/exclude"} {
		root := result.CommonDir
		if name == "config.worktree" {
			root = result.GitDir
		}
		data, err := readGitReviewFile(filepath.Join(root, filepath.FromSlash(name)), maxProjectFile)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
		if len(data) > maxProjectFile {
			return result, errors.New("Git review configuration exceeds 1 MiB.")
		}
		result.Metadata[name] = data
		fmt.Fprintf(h, "metadata:%s\x00%d\x00", name, len(data))
		h.Write(data)
	}
	untracked := map[string]bool{}
	for _, change := range parseGitChanges(result.Status) {
		if change.Status == "??" {
			untracked[change.Path] = true
		}
	}
	paths := strings.Split(files, "\x00")
	sort.Strings(paths)
	seen := map[string]bool{}
	total := int64(0)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if len(seen) > 10000 {
			return result, errors.New("Git review exceeds 10,000 files.")
		}
		if blockedProjectFile(path) && !journeyContextFile(path) {
			continue
		}
		result.Paths = append(result.Paths, path)
		full, err := guardedProjectPath(dir, path, false)
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(h, "deleted:%s\x00", path)
			continue
		}
		if err != nil {
			return result, err
		}
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			return result, errors.New("Git review requires ordinary files; symbolic links and submodules must be handled in Git.")
		}
		if info.Size() > (100<<20)-total {
			return result, errors.New("Git review exceeds 100 MiB.")
		}
		f, err := os.Open(full)
		if err != nil {
			return result, err
		}
		opened, statErr := f.Stat()
		if statErr != nil || !os.SameFile(info, opened) {
			f.Close()
			return result, errors.New("Source changed while opening. Refresh the Git review.")
		}
		data, readErr := io.ReadAll(io.LimitReader(f, info.Size()+1))
		after, afterErr := f.Stat()
		f.Close()
		if readErr != nil {
			return result, readErr
		}
		if afterErr != nil || int64(len(data)) != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return result, errors.New("Source changed while reading. Refresh the Git review.")
		}
		total += int64(len(data))
		fmt.Fprintf(h, "file:%s\x00%d\x00%d\x00", path, info.Mode().Perm()&0111, len(data))
		h.Write(data)
		if destination != "" {
			target := filepath.Join(destination, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return result, err
			}
			if err := os.WriteFile(target, data, 0600|(info.Mode().Perm()&0111)); err != nil {
				return result, err
			}
		}
		if untracked[path] && len(result.Preview) < maxProjectFile {
			result.Preview += "\nNew file: " + path + "\n"
			if utf8.Valid(data) && !strings.ContainsRune(string(data), '\x00') {
				remaining := maxProjectFile - len(result.Preview)
				if remaining > 0 {
					if len(data) > remaining {
						data = data[:remaining]
					}
					result.Preview += string(data)
				}
			} else {
				result.Preview += "[Binary file: contents are not displayed]\n"
			}
		}
	}
	result.Digest = hex.EncodeToString(h.Sum(nil))
	return result, nil
}

// Staging has its own immutable configuration/attributes and index, sharing
// only the validated object store. Publication separately uses a private
// detached HEAD and the common ref store while the real HEAD/index stay locked.
func preparePrivateReviewGit(parent string, review gitReview) (privateReviewGit, privateReviewGit, error) {
	stage := privateReviewGit{GitDir: filepath.Join(parent, "git"), Worktree: filepath.Join(parent, "worktree"), ObjectsDir: filepath.Join(review.CommonDir, "objects")}
	publish := privateReviewGit{GitDir: filepath.Join(parent, "publish"), Worktree: stage.Worktree}
	for _, dir := range []string{stage.GitDir, publish.GitDir, filepath.Join(stage.GitDir, "objects"), filepath.Join(stage.GitDir, "refs"), filepath.Join(stage.GitDir, "info")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return stage, publish, err
		}
	}
	head := review.Head
	if head == "" {
		head = strings.Repeat("0", review.hashLength())
	}
	for _, dir := range []string{stage.GitDir, publish.GitDir} {
		if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte(head+"\n"), 0600); err != nil {
			return stage, publish, err
		}
	}
	if review.SharedIndexName != "" {
		if err := os.WriteFile(filepath.Join(stage.GitDir, review.SharedIndexName), review.SharedIndex, 0600); err != nil {
			return stage, publish, err
		}
	}
	for name, data := range review.Metadata {
		if err := os.WriteFile(filepath.Join(stage.GitDir, filepath.FromSlash(name)), data, 0600); err != nil {
			return stage, publish, err
		}
	}
	if err := os.WriteFile(filepath.Join(publish.GitDir, "commondir"), []byte(filepath.ToSlash(review.CommonDir)+"\n"), 0600); err != nil {
		return stage, publish, err
	}
	return stage, publish, nil
}

func (review gitReview) hashLength() int {
	if review.ObjectFormat == "sha256" {
		return 64
	}
	return 40
}

func (s *Server) stageReviewedPaths(ctx context.Context, dir string, paths []string) error {
	// Literal pathspecs exclude unchanged protected entries in the copied index.
	// Batching also fits native Windows' command-line limit for long paths.
	for len(paths) > 0 {
		args := []string{"add", "--all", "--force", "--"}
		size := 0
		count := 0
		for count < len(paths) && (count == 0 || size+len(paths[count]) < 8000) {
			args = append(args, ":(literal)"+paths[count])
			size += len(paths[count]) + 12
			count++
		}
		if _, err := s.journeyGit(ctx, dir, args...); err != nil {
			return err
		}
		paths = paths[count:]
	}
	return nil
}

func readGitReviewFile(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > limit {
		return nil, errors.New("Git review metadata must be a bounded ordinary file.")
	}
	if err := hostpath.OneName(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.New("Git metadata changed during review.")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("Git review metadata exceeds its limit.")
	}
	return data, nil
}

// Generate the exact identity, timestamp and old/new object pair through Git,
// then prepare an owned replacement for the original per-worktree HEAD log.
// The caller already holds HEAD.lock and installs this only after ref CAS.
func (s *Server) prepareReviewedHeadLog(ctx context.Context, dir string, review gitReview, private privateReviewGit, commit, name, email, message string) (install func() error, cleanup func(), err error) {
	expected := review.Head
	if expected == "" {
		expected = strings.Repeat("0", review.hashLength())
	}
	args := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "-c", "user.useConfigOnly=true", "update-ref", "--create-reflog", "--no-deref", "-m", "commit: " + strings.Split(message, "\n")[0], "HEAD", commit, expected}
	if _, err = s.journeyGit(ctx, dir, args...); err != nil {
		return nil, nil, err
	}
	entry, err := readGitReviewFile(filepath.Join(private.GitDir, "logs", "HEAD"), 16384)
	if err != nil {
		return nil, nil, err
	}
	if !strings.HasPrefix(string(entry), expected+" "+commit+" ") || strings.Count(string(entry), "\n") != 1 {
		return nil, nil, errors.New("Git did not produce a single verified HEAD recovery entry.")
	}
	target := filepath.Join(review.GitDir, "logs", "HEAD")
	previous, err := readGitReviewFile(target, 16<<20)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return nil, nil, err
	}
	lock, err := os.OpenFile(target+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, nil, err
	}
	owned := true
	cleanup = func() {
		lock.Close()
		if owned {
			os.Remove(target + ".lock")
		}
	}
	if _, err = lock.Write(previous); err == nil {
		_, err = lock.Write(entry)
	}
	if err == nil {
		err = lock.Sync()
	}
	if closeErr := lock.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	install = func() error {
		if err := os.Rename(target+".lock", target); err != nil {
			return err
		}
		owned = false
		return nil
	}
	return install, cleanup, nil
}
