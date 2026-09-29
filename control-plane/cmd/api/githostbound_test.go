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

// cockpitCreateTimeout is CREATE_TIMEOUT_MS in desktop/src/api/client.ts: how
// long the cockpit lets POST /v1/sessions run before it abandons the create.
// Every host-config read runs inside it, in sequence, with the sandbox still
// to be made afterwards.
const cockpitCreateTimeout = 150 * time.Second

// createWorkReserve is what POST /v1/sessions still has to do once the three
// host-config reads are done, and it is why those reads may not spend the
// cockpit's whole window: the session record, then backend.Create — an image
// inspect, a container create, a start and an endpoint publish, four Docker
// API round trips on the same host whose PROCESS STARTS F2 measured at over
// 2.8 s each. The ready wait is not in here: handleSessionCreate hands that to
// watchSessionReady in a goroutine and answers the cockpit first.
//
// ARGUED, NOT MEASURED. The 2026-09-27 evidence records that the create
// succeeded, not how long its Docker half took, so this is a floor under the
// create's share rather than a measurement of it — and 20 s is what
// githostconfig.go's own arithmetic has always claimed to leave.
const createWorkReserve = 20 * time.Second

// Review C-1. A READ's budget and ONE git's bound are two decisions and they
// were one constant, so the four gits of a read shared
// project.HostConfigTimeout and got 7.5 s each — below the 10 s F2 proved too
// short. Six concurrent creates then turned a test the pre-change bounds pass
// 3/3 into a failure: the change made F2's own complaint more likely. The
// budget is sized from the READ, between the wall clock that read used to have
// and what one create can afford.
func TestAHostConfigReadsBudgetIsSizedFromTheReadAndNotFromOneGit(t *testing.T) {
	if hostConfigReadBudget <= project.HostConfigTimeout {
		t.Errorf("a whole read's budget is %s, no more than ONE git's bound %s, so the read's last git gets only what a single git's bound left it (C-1)",
			hostConfigReadBudget, project.HostConfigTimeout)
	}
	// It may not take away wall clock the read had before there was a budget:
	// the longest read is five gits, and each used to get project.GitTimeout.
	if had := 5 * project.GitTimeout; hostConfigReadBudget < had {
		t.Errorf("a whole read's budget is %s, less than the %s (5 × %s) the same gits had before the budget existed, so a slow read that used to finish now fails",
			hostConfigReadBudget, had, project.GitTimeout)
	}
	// And a create runs three reads in sequence before it makes a sandbox.
	//
	// Review 2, m-2: "they fit" is not the property. Merely fitting let the
	// budget reach 69 s with both suites green — three reads of 148 s inside a
	// 150 s window, leaving the create itself two seconds — so what is asserted
	// is the SHARE the reads may take, and the rest is named.
	if total := 2*hostConfigReadBudget + hostIdentityReadBudget; total > cockpitCreateTimeout-createWorkReserve {
		t.Errorf("the three host-config reads of one create can take %s, leaving the create's own work %s of the %s the cockpit allows: the reads are the PREAMBLE, and a create that is abandoned after them has done nothing a user can see",
			total, cockpitCreateTimeout-total, cockpitCreateTimeout)
	}
}

// Review I-3. The identity read's failure is not fatal — internal/api's
// sessionGitIdentity logs a warning and starts the session with an empty
// identity — so it must not make the user wait the budget that exists because
// a failure destroys the create. It had that budget only from a constant's
// name.
func TestTheIdentityReadIsNotBoundedLikeAReadThatCanFailACreate(t *testing.T) {
	if hostIdentityReadBudget >= hostConfigReadBudget {
		t.Errorf("the identity read waits %s, as long as a read whose failure destroys the create (%s), for a value the create throws away on failure",
			hostIdentityReadBudget, hostConfigReadBudget)
	}
	// Review M-4. Pinned only against the other budget, this number could be
	// loosened most of the way back without a red: 29 s passed every bound
	// test, one second under the per-git bound it was separated FROM. So pin
	// it to what it IS — a read of ONE git, whose answer is discarded on
	// failure, which also runs at startup before the control plane comes up.
	// A read like that may not outlast the bound a POLLED git gets, where a
	// failure costs one poll and nobody is waiting.
	if hostIdentityReadBudget > project.GitTimeout {
		t.Errorf("the identity read waits %s for one git whose answer is thrown away on failure, longer than the %s a git nobody waits for gets",
			hostIdentityReadBudget, project.GitTimeout)
	}
	// Review 2, m-5. Every assertion this constant had was an upper bound, so
	// it could fall as far as it liked: at 1 ms the identity read always spends
	// its budget, always fails and is always tolerated — an empty git identity
	// in every sandbox, behind one warning, with the whole cmd/api suite green.
	// It is ONE git, so it gets one git's bound; below that it is not a budget
	// but a guarantee of failure.
	if hostIdentityReadBudget < project.GitTimeout {
		t.Errorf("the identity read has %s for a git that may take %s, so the read can only fail: every sandbox would get an empty identity behind one warning",
			hostIdentityReadBudget, project.GitTimeout)
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

// hostConfigAnswer answers every git any of the three resolvers asks, so a
// read reaches its end and every git it makes is recorded.
func hostConfigAnswer(args []string) ([]byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "--default "+hostGitUnset):
		return []byte(hostGitUnset + "\n"), nil
	case strings.Contains(joined, "GIT_AUTHOR_IDENT"):
		return []byte("Someone <someone@example.com> 1790156728 +1000\n"), nil
	case strings.Contains(joined, "-z --show-origin"):
		// The one -z record valueWithOrigin accepts (X-3 review 4 I-C), here
		// --default's answer, so no origin file is named.
		return []byte("command line:\x00false\x00"), nil
	}
	return []byte("false\n"), nil
}

// Raising one git's bound to 30 s would let a create hang for the sum of the
// four or five gits a resolver runs. It cannot: the resolver bounds the WHOLE
// read once, and every git shares that one deadline.
//
// Review I-2: this was asserted for the line-endings read alone, so deleting
// the budget from gitGlobalFilesResolver — the read with the MOST gits, and
// the one whose failure is actually on record — left the whole cmd/api suite
// green. Every resolver is asserted here, each against its own budget.
func TestEveryHostConfigReadIsBoundedInTotalAndNotPerGit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		budget time.Duration
		gits   int
		read   func(hostGit) error
	}{
		{"line endings", hostConfigReadBudget, 4, func(h hostGit) error {
			_, err := gitLineEndingsResolverWith(h, "lf")(context.Background())
			return err
		}},
		{"global files", hostConfigReadBudget, 4, func(h hostGit) error {
			_, err := gitGlobalFilesResolver(h)(context.Background())
			return err
		}},
		{"identity", hostIdentityReadBudget, 1, func(h hostGit) error {
			_, err := gitIdentityResolver("", "", h)(context.Background())
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &deadlineRunner{answer: hostConfigAnswer}
			before := time.Now().Add(tc.budget)
			if err := tc.read(boundedHost(r)); err != nil {
				t.Fatalf("resolve: %v", err)
			}
			after := time.Now().Add(tc.budget)

			if len(r.deadlines) < tc.gits {
				t.Fatalf("%d gits ran, want the %d this read makes", len(r.deadlines), tc.gits)
			}
			for i, d := range r.deadlines {
				if d.IsZero() {
					t.Fatalf("git %d ran with no deadline; the read is unbounded in total", i)
				}
				if d.Before(before) || d.After(after) {
					t.Errorf("git %d has deadline %s, outside the one %s budget [%s, %s]",
						i, d, tc.budget, before, after)
				}
				if !d.Equal(r.deadlines[0]) {
					t.Errorf("git %d has its own deadline %s, not the read's %s: the bounds add up",
						i, d, r.deadlines[0])
				}
			}
		})
	}
}

// Review M-2. githostconfig.go claimed the runner tells a spent budget from a
// request that was already over. It cannot: from project.ExecRunner's side the
// budget IS the caller, so both produced "the caller's deadline passed …
// inside the 30s bound" and only GitError.Elapsed separated them, by
// inference. The read makes the distinction itself, where it is knowable.
func TestAHostConfigReadSaysWhenItWasItsOwnBudgetThatRanOut(t *testing.T) {
	h := boundedHost(&deadlineRunner{answer: hostConfigAnswer})

	spent := h.read(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if spent == nil {
		t.Fatal("a read that outlasted its budget returned no error")
	}
	if !errors.Is(spent, context.DeadlineExceeded) {
		t.Errorf("error %q does not wrap context.DeadlineExceeded", spent)
	}
	if msg := spent.Error(); !strings.Contains(msg, "spent its whole 20ms budget") {
		t.Errorf("error = %q, want it to name the budget this read spent", msg)
	}

	over, cancel := context.WithCancel(context.Background())
	cancel()
	gone := h.read(over, time.Minute, func(ctx context.Context) error { return ctx.Err() })
	if gone == nil {
		t.Fatal("a read whose caller had already gone returned no error")
	}
	if msg := gone.Error(); strings.Contains(msg, "budget") {
		t.Errorf("error = %q; the request was already over, and this read's budget is not what ended it", msg)
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
