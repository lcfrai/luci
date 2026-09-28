package contextwrite

import (
	"context"
	"errors"
)

// Git is the three questions this package asks git. None of them writes: no
// git command here adds, commits or updates the index. They are not "safe"
// merely for being read-only, though: the worktree is bind-mounted into the
// sandbox read-write, .git/config included (api/sessions.go:343), so the
// agent owns the repository's config, and host git applies it. NewGit
// (gitcmd.go) is the one implementation, over project.Runner, and it disables
// the config that turns these commands into a program start on the host
// (core.fsmonitor, review I-1). Each command runs with <dir> as its working
// directory (Run's dir argument; no -C is passed):
//
//	Tracked:     git -c core.fsmonitor=false --icase-pathspecs ls-files -- <rel>
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
//	ExcludePath: git -c core.fsmonitor=false rev-parse --git-path info/exclude
//	             (joined to dir if relative).
//	Ignored:     git -c core.fsmonitor=false check-ignore -v -- <rel>
//	             asked after step 6 wrote its line (review m1): whether git
//	             now ignores <rel>. -v prints the last pattern that matches,
//	             a negation ("!CLAUDE.md" in a .gitignore, which outranks
//	             info/exclude) included, and exits 0 whenever any pattern
//	             matches; step 6's own line matches in a worktree that is the
//	             repository's top level, so exit 1 (no pattern matches: the
//	             worktree is a subdirectory) arrives as the runner's error, a
//	             500. Without -v a negation exits 1 too, and "not ignored"
//	             could not be told from a failure through the runner.
//
// rel is slash-separated and relative to dir: FileName or AltFileName.
//
// NO exit code is read. project.ExecRunner.Run wraps a failure with %v
// (gitrunner.go:79-80), which drops the *exec.ExitError, so "exit 1 means
// untracked" cannot be seen through the runner, and --error-unmatch is not
// used. The one error text parsed is "not a git repository", from any of the
// three, which becomes ErrNotARepo; the plan knows it is a string match.
// The exclude FILE is written by this package with statepath.WriteAtomic,
// which is a file write, not a git command.
type Git interface {
	Tracked(ctx context.Context, dir, rel string) (bool, error)
	ExcludePath(ctx context.Context, dir string) (string, error)
	Ignored(ctx context.Context, dir, rel string) (bool, error)
}

// ErrNotARepo: dir is not inside a git repository. Returned, or wrapped, by
// a Git; this package then treats both candidates as strippable, targets
// FileName and writes no exclude line.
var ErrNotARepo = errors.New("contextwrite: not inside a git repository")
