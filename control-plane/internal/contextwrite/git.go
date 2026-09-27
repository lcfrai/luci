package contextwrite

import (
	"context"
	"errors"
)

// Git is the two read-only questions this package asks git. Task 14 wires it
// over project.Runner (phase 2's read-only runner), each command run with
// <dir> as its working directory (Run's dir argument; no -C is passed):
//
//	Tracked:     git --icase-pathspecs ls-files -- <rel>
//	             (exit 0 either way: empty output = untracked, any output =
//	             tracked; review I7). --icase-pathspecs because a pathspec is
//	             case-sensitive even under core.ignorecase=true (the Windows
//	             default): plain `ls-files -- CLAUDE.md` prints nothing when
//	             the index holds claude.md, yet opening CLAUDE.md on NTFS or
//	             APFS opens that tracked file (re-review R-I1). Not the
//	             :(icase) pathspec magic: under GIT_LITERAL_PATHSPECS=1 that
//	             silently matches nothing ("untracked"), where the option
//	             fails with exit 128, which is a 500. On a case-sensitive
//	             Linux checkout a tracked claude.md now reads as tracked and
//	             moves the target to .claude/CLAUDE.md.
//	ExcludePath: git rev-parse --git-path info/exclude  (joined to dir if relative)
//
// rel is slash-separated and relative to dir: FileName or AltFileName.
//
// NO exit code is read. project.ExecRunner.Run wraps a failure with %v
// (gitrunner.go:74), which drops the *exec.ExitError, so "exit 1 means
// untracked" cannot be seen through the runner, and --error-unmatch is not
// used. The one error text parsed is "not a git repository", from either
// command, which becomes ErrNotARepo; the plan knows it is a string match.
// It never writes through git. The exclude FILE is written by this package
// with statepath.WriteAtomic, which is a file write, not a git command.
type Git interface {
	Tracked(ctx context.Context, dir, rel string) (bool, error)
	ExcludePath(ctx context.Context, dir string) (string, error)
}

// ErrNotARepo: dir is not inside a git repository. Returned, or wrapped, by
// a Git; this package then treats both candidates as strippable, targets
// FileName and writes no exclude line.
var ErrNotARepo = errors.New("contextwrite: not inside a git repository")
