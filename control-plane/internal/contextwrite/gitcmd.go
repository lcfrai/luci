package contextwrite

import (
	"context"
	"fmt"
	"strings"
)

// Runner is project.Runner's one method: git with args, run in dir, its
// standard output returned. A failure is an error carrying git's own
// sentence (project.ExecRunner puts stderr in it, gitrunner.go:74-80). Task
// 14 passes project.NewExecRunner(); nothing here imports project.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// NewGit is Git over r, each question one command (git.go names them).
func NewGit(r Runner) Git { return runnerGit{r: r} }

type runnerGit struct{ r Runner }

var _ Git = runnerGit{}

// hardened comes first in every command this package runs (review I-1). The
// agent owns .git/config (git.go), and a repository-local
// core.fsmonitor = <command> makes host git run <command> on the HOST,
// outside the sandbox, from ls-files and from check-ignore alike (measured
// on git 2.53.0.windows.2: review I-1's fsm.out, and this round's probe;
// rev-parse --git-path reads no index and ran nothing, and carries it all
// the same). A -c on the command line outranks every config file and every
// include, so "false" here is final. TestNoGitQuestionStartsTheRepositorysFsmonitor
// plants the hook over real git.
//
// UNVERIFIED beyond core.fsmonitor: that none of the three commands starts
// any other program named by config. The ones git has - a pager, a
// credential helper, hooks, a clean or smudge filter, an ssh or proxy
// command, a diff or textconv driver - belong to commands that read or write
// content or talk to a remote, which these do not; a pager also needs a
// terminal, and project.ExecRunner captures standard output.
var hardened = []string{"-c", "core.fsmonitor=false"}

func gitArgs(rest ...string) []string {
	return append(append(make([]string, 0, len(hardened)+len(rest)), hardened...), rest...)
}

func (g runnerGit) Tracked(ctx context.Context, dir, rel string) (bool, error) {
	out, err := g.r.Run(ctx, dir, gitArgs("--icase-pathspecs", "ls-files", "--", rel)...)
	if err != nil {
		return false, gitError(err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func (g runnerGit) ExcludePath(ctx context.Context, dir string) (string, error) {
	out, err := g.r.Run(ctx, dir, gitArgs("rev-parse", "--git-path", "info/exclude")...)
	if err != nil {
		return "", gitError(err)
	}
	p := strings.TrimRight(string(out), "\r\n")
	if p == "" || strings.ContainsAny(p, "\r\n") {
		return "", fmt.Errorf("contextwrite: git rev-parse --git-path info/exclude printed %q, not one path", out)
	}
	return p, nil
}

func (g runnerGit) Ignored(ctx context.Context, dir, rel string) (bool, error) {
	out, err := g.r.Run(ctx, dir, gitArgs("check-ignore", "-v", "--", rel)...)
	if err != nil {
		return false, gitError(err)
	}
	return parseCheckIgnore(string(out), rel)
}

// parseCheckIgnore reads `check-ignore -v -- <rel>`'s one line,
// <source>:<line number>:<pattern><TAB><path>, and answers whether <pattern>
// ignores <rel>: it does unless it is a negation, which -v prints with its
// "!". The source may hold a colon (a Windows drive: core.excludesFile is
// C:/Users/...), so the pattern starts after the FIRST ":<digits>:". A
// source holding ":<digits>:" itself would be misread; no source git names
// here does (.gitignore, .claude/.gitignore, the exclude file, the person's
// excludes file).
func parseCheckIgnore(out, rel string) (bool, error) {
	bad := fmt.Errorf("contextwrite: git check-ignore -v -- %s printed %q, not <source>:<line>:<pattern><TAB>%s", rel, out, rel)
	line := strings.TrimRight(out, "\r\n")
	tab := strings.LastIndexByte(line, '\t')
	if tab < 0 || strings.ContainsAny(line, "\r\n") || line[tab+1:] != rel {
		return false, bad
	}
	head := line[:tab]
	for i := 0; i < len(head); i++ {
		if head[i] != ':' {
			continue
		}
		j := i + 1
		for j < len(head) && head[j] >= '0' && head[j] <= '9' {
			j++
		}
		if j > i+1 && j < len(head) && head[j] == ':' {
			pattern := head[j+1:]
			if pattern == "" {
				return false, bad
			}
			return !strings.HasPrefix(pattern, "!"), nil
		}
	}
	return false, bad
}

// gitError is ErrNotARepo for git's "not a git repository", else err as it
// is: a 500 at the handler, never a guess.
func gitError(err error) error {
	if strings.Contains(err.Error(), "not a git repository") {
		return fmt.Errorf("%w: %v", ErrNotARepo, err)
	}
	return err
}
