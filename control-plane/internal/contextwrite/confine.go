package contextwrite

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// THE LINK RULE (review I-2). Every file this package reads or writes lies
// in a tree the sandbox writes: the worktree is bind-mounted into it
// read-write, .git included (api/sessions.go:343). A link there is the
// agent's to plant, and the HOST resolves it against the host's filesystem.
// A CLAUDE.md linking to ../../.ssh/id_ed25519 would be read into the
// brief's file, where the sandbox reads it; a .claude linking to the
// person's ~/.claude would take the brief's write, or a strip, into their
// own global CLAUDE.md. Phase 2 refuses the same for .git
// (api/sessions.go:579-582).
//
// So nothing below the worktree is followed. Before every read and every
// write, each directory between the worktree and the file must be a real
// directory - not a symlink, and on Windows not a junction or another
// reparse point (Go 1.23's Lstat reports a mount point ModeIrregular, which
// is not ModeDir) - and the file itself a regular file or absent. A read
// then opens the file and checks with os.SameFile that it opened the file
// it checked, so a link swapped in after the check, for the file or for a
// directory above it, is refused, not followed; on Linux and macOS the open
// is O_NOFOLLOW|O_NONBLOCK too, so a FIFO swapped in cannot hold the start
// open (open_unix.go). Every write
// re-checks its path just before statepath.WriteAtomic, because step 6's
// git commands run between step 5's plan and step 7's writes.
//
// NOT closed: a directory swapped for a link between a write's check and
// WriteAtomic's rename. The restart path materialises while the old agent
// still runs (api/sessions.go:1724), so the window is real, if narrow, and
// only below .claude or .git: CLAUDE.md's own directory is the worktree, the
// mount's root, which a container cannot rename or replace. Go 1.24's
// os.Root would close it; go.mod stays at go 1.23.

// checkPath applies the link rule to path and returns its Lstat, nil when it
// or a directory above it is absent. A path below dir is checked from dir
// down; any other (only an exclude file can be: a linked worktree's common
// directory) takes checkOutside.
func checkPath(dir, path string) (fs.FileInfo, error) {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return checkOutside(path)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	cur := dir
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, nil
		case err != nil:
			return nil, err
		case i < len(parts)-1 && fi.Mode().Type() != fs.ModeDir:
			return nil, notFollowed(cur, fi, "a directory")
		case i == len(parts)-1 && !fi.Mode().IsRegular():
			return nil, notFollowed(cur, fi, "a regular file")
		case i == len(parts)-1:
			return fi, nil
		}
	}
	return nil, nil
}

// checkOutside is the rule for an exclude file git places outside the
// worktree. What lies above it is the host's, so only the file, its info
// directory and the git directory holding that are checked, and that last
// must exist: WriteAtomic's MkdirAll then makes at most the info directory,
// never a tree. The residue, named: a planted gitfile or commondir can steer
// git's answer to another existing directory's info/exclude, which then
// gains this package's two fixed lines.
func checkOutside(path string) (fs.FileInfo, error) {
	info := filepath.Dir(path)
	gitDir := filepath.Dir(info)
	fi, err := os.Lstat(gitDir)
	if err != nil {
		return nil, err // absent included: no tree is built outside the worktree
	}
	if fi.Mode().Type() != fs.ModeDir {
		return nil, notFollowed(gitDir, fi, "a directory")
	}
	fi, err = os.Lstat(info)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, err
	case fi.Mode().Type() != fs.ModeDir:
		return nil, notFollowed(info, fi, "a directory")
	}
	fi, err = os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, err
	case !fi.Mode().IsRegular():
		return nil, notFollowed(path, fi, "a regular file")
	}
	return fi, nil
}

func notFollowed(path string, fi fs.FileInfo, want string) error {
	what := "something else"
	switch t := fi.Mode().Type(); {
	case t&fs.ModeSymlink != 0:
		what = "a link"
	case t&fs.ModeIrregular != 0:
		what = "a link or another reparse point"
	case t == fs.ModeDir:
		what = "a directory"
	case t == 0:
		what = "a file"
	}
	return fmt.Errorf("%s is %s, not %s, and is not followed", path, what, want)
}

// openForRead is openNoFollow. It is a variable only so that
// confine_internal_test.go can swap a link in between readChecked's check and
// its open, a moment no other test reaches.
var openForRead = openNoFollow

// readChecked reads path under the link rule: "" and false when absent.
func readChecked(dir, path string) (string, bool, error) {
	want, err := checkPath(dir, path)
	if err != nil || want == nil {
		return "", false, err
	}
	// Pin want's identity now, while the checked path is the checked file.
	// On Windows an Lstat's file ID is read lazily, by path, the first time
	// os.SameFile needs it; read after the open, through a directory swapped
	// for a link in between, it would name the outside file the open reached,
	// and the two would agree. SameFile of want with itself loads it here. On
	// Linux and macOS the ID came with the Lstat, and this costs nothing.
	_ = os.SameFile(want, want)
	f, err := openForRead(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil // removed since the check: absent
	}
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	got, err := f.Stat()
	if err != nil {
		return "", false, err
	}
	if !got.Mode().IsRegular() || !os.SameFile(want, got) {
		return "", false, fmt.Errorf("%s changed while it was being opened, and is not followed", path)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}
