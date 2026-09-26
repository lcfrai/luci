package lanes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
)

// LocalFS is gitx.MachineFS on this process's own disk (seam S8-P3.6): the os calls this task
// makes, moved behind the interface unchanged.
//   - Exists is os.Lstat: fs.ErrNotExist → (false, false, nil); success → (true, dir, nil),
//     dir true only for a real directory (Mode().Type() == fs.ModeDir), so a symlink or a
//     junction is exists and not dir (Go 1.23+ reports a junction as ModeDir|ModeIrregular, so
//     IsDir() alone would call it a directory); any other error is returned;
//   - RemoveTree is RemoveTree(root, path);
//   - CopyGeneratedHooks is the copy half of lanes.CopyGeneratedHooks: the `.git`-segment
//     refusal again (env task 21's copy-hooks route serves through LocalFS, so it inherits it),
//     the not-exist Lstat of <cloneTop>/<rel> and the real-directory walk of its existing
//     parents, the real-directory walk of the project side (none of whose components may be
//     the project's own .git by os.SameFile), SameFile at open and the 64-file / 1 MiB budget;
//   - Seal returns nil and touches nothing.
//
// Each method returns ctx.Err() when ctx has ended before its first os call.
type LocalFS struct{}

var _ gitx.MachineFS = LocalFS{}

// ErrNoMachineFS: a machine whose ID is not "local" answered a nil FS. Nothing was touched.
var ErrNoMachineFS = errors.New("lanes: this machine has no filesystem for lane create and remove")

// FSOf is the one place a MachineFS is taken from a machine: m.FS() when non-nil; LocalFS{}
// when it is nil and m.ID() == "local"; otherwise ErrNoMachineFS, so a cloud machine never
// falls back to the laptop's disk. Create, CopyGeneratedHooks and task 11's Remove call it.
func FSOf(m gitx.Machine) (gitx.MachineFS, error) {
	if f := m.FS(); f != nil {
		return f, nil
	}
	if m.ID() == "local" {
		return LocalFS{}, nil
	}
	return nil, fmt.Errorf("%w: machine %s", ErrNoMachineFS, m.ID())
}

// realDir is a real directory by Lstat: no symlink, junction or other reparse point.
func realDir(fi fs.FileInfo) bool { return fi.Mode().Type() == fs.ModeDir }

func (LocalFS) Exists(ctx context.Context, path string) (exists, dir bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, false, err
	}
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, false, nil
	case err != nil:
		return false, false, err
	}
	return true, realDir(fi), nil
}

func (LocalFS) RemoveTree(ctx context.Context, root, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return RemoveTree(root, path)
}

func (LocalFS) Seal(ctx context.Context, path string) error { return nil }

// The copy budget (review Minor 9).
const (
	maxHookFiles = 64
	maxHookBytes = 1 << 20
)

// copySource is what openForCopy answers: *os.File satisfies it.
type copySource interface {
	io.ReadCloser
	Stat() (fs.FileInfo, error)
}

// openForCopy is a test-only seam (TestCopyGeneratedHooksChecksTheOpenedFile): production
// opens the path.
var openForCopy = func(name string) (copySource, error) { return os.Open(name) }

// hasGitSegment is the `.git`-segment refusal: a segment that is `.git` case-insensitively
// after trimming the trailing dots and spaces Windows drops.
func hasGitSegment(rel string) bool {
	for _, seg := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.EqualFold(strings.TrimRight(seg, ". "), ".git") {
			return true
		}
	}
	return false
}

func (LocalFS) CopyGeneratedHooks(ctx context.Context, projectTop, cloneTop, rel string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	segs := relSegments(rel)
	if segs == nil || hasGitSegment(rel) {
		return 0, nil
	}
	src, ok := projectHooksDir(projectTop, segs)
	if !ok {
		return 0, nil
	}
	dst, ok := cloneHooksDir(cloneTop, segs)
	if !ok {
		return 0, nil
	}
	files, ok := hookFiles(src)
	if !ok || len(files) == 0 {
		return 0, nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, fmt.Errorf("lanes: hooks directory %s: %w", dst, err)
	}
	if err := copyHookFiles(src, dst, files); err != nil {
		_ = RemoveTree(cloneTop, dst) // dst did not exist before this call
		return 0, err
	}
	return len(files), nil
}

// projectHooksDir walks <projectTop>/<segs>: every component below projectTop, the last one
// included, is a real directory by Lstat and none is the project's own .git by os.SameFile, which
// catches an 8.3 short name (GIT~1) the segment compare cannot see (carried b03-2 Minor 2).
func projectHooksDir(projectTop string, segs []string) (string, bool) {
	gitFi, gitErr := os.Lstat(filepath.Join(projectTop, ".git"))
	p := projectTop
	for _, seg := range segs {
		p = filepath.Join(p, seg)
		fi, err := os.Lstat(p)
		if err != nil || !realDir(fi) {
			return "", false
		}
		if gitErr == nil && os.SameFile(fi, gitFi) {
			return "", false
		}
	}
	return p, true
}

// cloneHooksDir is <cloneTop>/<segs> when it does not exist and every existing parent of it,
// cloneTop included, is a real directory by Lstat (carried b03-2 Minor 3): a lane branch that
// tracks a parent as a link must not route the copy, or its take-back, outside the clone.
func cloneHooksDir(cloneTop string, segs []string) (string, bool) {
	if fi, err := os.Lstat(cloneTop); err != nil || !realDir(fi) {
		return "", false
	}
	p := cloneTop
	for i, seg := range segs {
		p = filepath.Join(p, seg)
		fi, err := os.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return filepath.Join(append([]string{cloneTop}, segs...)...), true
		case err != nil, i == len(segs)-1, !realDir(fi):
			return "", false // an Lstat error, a destination that exists, or a parent that is a link
		}
	}
	return "", false
}

type hookFile struct {
	name string
	fi   fs.FileInfo
}

// hookFiles lists src when it holds only regular files, at most 64 and 1 MiB in total.
func hookFiles(src string) ([]hookFile, bool) {
	ents, err := os.ReadDir(src)
	if err != nil || len(ents) > maxHookFiles {
		return nil, false
	}
	var total int64
	files := make([]hookFile, 0, len(ents))
	for _, e := range ents {
		fi, err := os.Lstat(filepath.Join(src, e.Name()))
		if err != nil || !fi.Mode().IsRegular() {
			return nil, false
		}
		if total += fi.Size(); total > maxHookBytes {
			return nil, false
		}
		files = append(files, hookFile{name: e.Name(), fi: fi})
	}
	return files, true
}

// copyHookFiles opens each file, requires os.SameFile with its Lstat and the same size, and
// reads it through the budget that remains, so a swap or a growth after the check is refused
// and nothing past 1 MiB in total is read (review Minor 9).
func copyHookFiles(src, dst string, files []hookFile) error {
	var read int64
	for _, h := range files {
		p := filepath.Join(src, h.name)
		f, err := openForCopy(p)
		if err != nil {
			return fmt.Errorf("lanes: open hook %s: %w", p, err)
		}
		ofi, err := f.Stat()
		if err != nil || !os.SameFile(h.fi, ofi) || ofi.Size() != h.fi.Size() {
			f.Close()
			return fmt.Errorf("lanes: hook %s changed between its check and its open", p)
		}
		data, err := io.ReadAll(io.LimitReader(f, maxHookBytes-read))
		f.Close()
		read += int64(len(data))
		if err != nil {
			return fmt.Errorf("lanes: read hook %s: %w", p, err)
		}
		if int64(len(data)) != h.fi.Size() {
			return fmt.Errorf("lanes: hook %s is not the size it was checked at", p)
		}
		out, err := os.OpenFile(filepath.Join(dst, h.name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, h.fi.Mode().Perm())
		if err != nil {
			return fmt.Errorf("lanes: write hook: %w", err)
		}
		_, werr := out.Write(data)
		if cerr := out.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return fmt.Errorf("lanes: write hook: %w", werr)
		}
	}
	return nil
}

// relSegments is rel's segments, or nil when rel is not a plain relative path under its top.
func relSegments(rel string) []string {
	if rel == "" || strings.ContainsAny(rel, ":\x00") || filepath.IsAbs(rel) || rel[0] == '/' || rel[0] == '\\' {
		return nil
	}
	var segs []string
	for _, seg := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		switch seg {
		case ".":
			continue
		case "..":
			return nil
		}
		segs = append(segs, seg)
	}
	return segs
}
