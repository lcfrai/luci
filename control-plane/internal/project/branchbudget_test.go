package project_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// budgetGit records the deadline every git of a read is given.
type budgetGit struct {
	mu        sync.Mutex
	deadlines []time.Time
	args      []string
}

func (g *budgetGit) Run(ctx context.Context, _ string, args ...string) ([]byte, error) {
	g.mu.Lock()
	d, _ := ctx.Deadline()
	g.deadlines = append(g.deadlines, d)
	joined := strings.Join(args, " ")
	g.args = append(g.args, joined)
	g.mu.Unlock()

	if strings.HasPrefix(joined, "for-each-ref") {
		return []byte("main\nfeat/ready-flag\n"), nil
	}
	// The abbrev-ref fails so the confirmation runs too: three gits, which is
	// the worst case F4 created.
	return nil, &project.GitError{Args: args, Err: errors.New("exit status 128"), Stderr: "fatal: not a git repository: .git"}
}

var _ project.Runner = (*budgetGit)(nil)

// Review M-3. F4 gave Branches a third git — for-each-ref, rev-parse
// --abbrev-ref and the unborn-HEAD confirmation — and each took its own
// GitTimeout, so the server-side worst case for GET
// /v1/projects/{id}/branches went from 20 s to 30 s while GitTimeout's own
// stated reason is that a blocked git must not hold a request open. The gits
// of one read share one deadline, and three of them cannot add up past one
// git's bound. Nothing asserted this before: the read is bounded in total.
func TestABranchReadIsBoundedInTotalAndNotPerGit(t *testing.T) {
	root := t.TempDir()
	dir := makeRepo(t, root, "panacea")
	g := &budgetGit{}

	start := time.Now()
	if _, err := branchesOf(t, root, dir, g); err == nil {
		t.Fatal("Branches with a refused rev-parse returned no error")
	}
	latest := time.Now().Add(project.GitTimeout)

	if len(g.deadlines) < 3 {
		t.Fatalf("%d gits ran (%v), want the three a branch read makes on this path", len(g.deadlines), g.args)
	}
	for i, d := range g.deadlines {
		if d.IsZero() {
			t.Fatalf("git %d (%s) ran with no deadline; the read is unbounded in total", i, g.args[i])
		}
		if !d.Equal(g.deadlines[0]) {
			t.Errorf("git %d (%s) has its own deadline %s, not the read's %s: the bounds add up",
				i, g.args[i], d, g.deadlines[0])
		}
		if d.Before(start) || d.After(latest) {
			t.Errorf("git %d (%s) has deadline %s, outside the one %s the whole read gets [%s, %s]",
				i, g.args[i], d, project.GitTimeout, start, latest)
		}
	}
}
