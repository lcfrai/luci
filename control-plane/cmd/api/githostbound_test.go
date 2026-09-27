package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// boundedHost is a hostGit over any Runner, for the bound's own tests.
func boundedHost(r project.Runner) hostGit {
	return hostGit{run: r, root: os.TempDir(), getenv: func(string) string { return "" }}
}

// F2 (b). A host-config git is not an enumeration git. Enumeration sits behind
// a handler the rail polls, so it takes GitTimeout and a failure costs one
// poll; a host-config read runs once inside POST /v1/sessions and its failure
// destroys the create. On 2026-09-27 one exceeded 10 s on a quiet host.
func TestHostConfigGitIsBoundedForACreateAndNotForAPoll(t *testing.T) {
	h, err := newHostGit()
	if err != nil {
		t.Fatalf("newHostGit: %v", err)
	}
	r, ok := h.run.(*project.ExecRunner)
	if !ok {
		t.Fatalf("host git runner is %T, want *project.ExecRunner", h.run)
	}
	if r.Timeout != project.HostConfigTimeout {
		t.Errorf("host git bound = %s, want project.HostConfigTimeout %s", r.Timeout, project.HostConfigTimeout)
	}
	if project.HostConfigTimeout <= project.GitTimeout {
		t.Errorf("HostConfigTimeout %s does not outlast the poll bound %s, so it is not its own decision",
			project.HostConfigTimeout, project.GitTimeout)
	}
}

// deadlineRunner records the deadline each git is given.
type deadlineRunner struct {
	mu        sync.Mutex
	deadlines []time.Time
	answer    func(args []string) ([]byte, error)
}

func (r *deadlineRunner) Run(ctx context.Context, _ string, args ...string) ([]byte, error) {
	r.mu.Lock()
	if d, ok := ctx.Deadline(); ok {
		r.deadlines = append(r.deadlines, d)
	} else {
		r.deadlines = append(r.deadlines, time.Time{})
	}
	r.mu.Unlock()
	return r.answer(args)
}

var _ project.Runner = (*deadlineRunner)(nil)

// Raising one git's bound to 30 s would let a create hang for the sum of the
// four or five gits a resolver runs. It cannot: the resolver bounds the WHOLE
// read once, and every git shares that one deadline.
func TestAHostConfigReadIsBoundedInTotalAndNotPerGit(t *testing.T) {
	r := &deadlineRunner{answer: func(args []string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "--default "+hostGitUnset) {
			return []byte(hostGitUnset + "\n"), nil
		}
		return []byte("false\n"), nil
	}}
	before := time.Now().Add(project.HostConfigTimeout)

	if _, err := gitLineEndingsResolverWith(boundedHost(r), "lf")(context.Background()); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	after := time.Now().Add(project.HostConfigTimeout)
	if len(r.deadlines) < 2 {
		t.Fatalf("%d gits ran, want the several a line-endings read makes", len(r.deadlines))
	}
	for i, d := range r.deadlines {
		if d.IsZero() {
			t.Fatalf("git %d ran with no deadline; the read is unbounded in total", i)
		}
		if d.Before(before) || d.After(after) {
			t.Errorf("git %d has deadline %s, outside the one %s budget [%s, %s]",
				i, d, project.HostConfigTimeout, before, after)
		}
		if !d.Equal(r.deadlines[0]) {
			t.Errorf("git %d has its own deadline %s, not the read's %s: the bounds add up",
				i, d, r.deadlines[0])
		}
	}
}

// F2 (c). A read that was killed must fail the create and say so. It must NOT
// fall back to git's default: the sandbox's git would then convert line
// endings differently from the host's, which rewrites every file the agent
// touches and is caught by nothing downstream. A failed create is loud, costs
// one retry and loses no work.
func TestAKilledLineEndingsReadFailsTheCreateAndNamesTheBound(t *testing.T) {
	killed := &project.GitError{
		Args:     []string{"config", "--get", "--system", "--includes", "--default", "native", "core.eol"},
		Dir:      "C:\\Users\\AU-DEV~1\\AppData\\Local\\Temp\\panacea-app-hostgit-1400797516",
		Err:      errors.New("exit status 1"),
		Timeout:  project.HostConfigTimeout,
		Elapsed:  project.HostConfigTimeout,
		TimedOut: true,
	}
	r := &eolRunner{answers: eolAnswers(map[string]eolAnswer{
		eolAskBool: {out: "true\n"},
		eolAskEOL:  {err: killed},
	})}

	got, err := gitLineEndingsResolverWith(scriptedHost(r, func(string) string { return "" }), "crlf")(context.Background())
	if err == nil {
		t.Fatalf("a killed core.eol read resolved to %#v; the sandbox must not be given a guess", got)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "timed out") || !strings.Contains(msg, project.HostConfigTimeout.String()) {
		t.Errorf("error = %q, want the timeout and the bound it exceeded named", msg)
	}
	if got != (api.GitLineEndings{}) {
		t.Errorf("line endings = %#v alongside an error, want none", got)
	}
}
