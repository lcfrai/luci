package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type coordinatorGitFixture struct {
	bare         string
	pushes       atomic.Int32
	observations atomic.Int32
	loseACK      atomic.Bool
}

// Only the already validated HTTPS transport is translated to this test-owned
// local bare remote. The actual Git push/ref observation runs, without account
// access, network requests, new repositories outside t.TempDir or provider work.
func installCoordinatorGitFixture(t *testing.T, s *Server, p project.Project) *coordinatorGitFixture {
	t.Helper()
	f := &coordinatorGitFixture{bare: filepath.Join(t.TempDir(), "destination.git")}
	if _, err := RunCommand(context.Background(), "git", []string{"init", "--bare", f.bare}, ""); err != nil {
		t.Fatal(err)
	}
	const address = "https://github.com/example/coordinator.git"
	if _, err := s.journeyGit(context.Background(), p.Path, "remote", "add", "origin", address); err != nil {
		t.Fatal(err)
	}
	original := s.runner
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		command := ""
		if exe == "git" {
			for _, a := range args {
				if a == "push" || a == "ls-remote" {
					command = a
					break
				}
			}
		}
		if command == "" {
			return original(ctx, exe, args, dir)
		}
		s.mu.RLock()
		rows, err := s.readContinuations(p, "alice")
		s.mu.RUnlock()
		if err != nil || len(rows.Rows) != 1 || rows.Rows[0].Delivery == nil || rows.Rows[0].Delivery.GitPublication == nil {
			return "", errors.New("Git network operation preceded durable exact reservation")
		}
		action := rows.Rows[0].Delivery.GitPublication
		if action.ActionID != rows.Rows[0].ID+"-git" || action.Repository != address || action.Branch != "main" {
			return "", errors.New("wrong Git authority binding")
		}
		replacement := append([]string{}, args...)
		found := false
		for i, a := range replacement {
			if a == address {
				replacement[i] = f.bare
				found = true
			}
		}
		if !found {
			return "", errors.New("unexpected Git network destination")
		}
		if command == "push" {
			for _, required := range []string{"--no-force", "--no-mirror", "--no-follow-tags", "--recurse-submodules=no", action.Commit + ":refs/heads/main"} {
				present := false
				for _, a := range args {
					present = present || a == required
				}
				if !present {
					return "", errors.New("missing exact safe push option")
				}
			}
			f.pushes.Add(1)
		} else {
			f.observations.Add(1)
		}
		replacement = append([]string{"-c", "protocol.file.allow=always"}, replacement...)
		out, err := RunCommand(ctx, exe, replacement, dir)
		if command == "push" && err == nil && f.loseACK.Load() {
			return "", errors.New("controlled ACK loss after actual bare remote mutation")
		}
		return out, err
	}
	return f
}

func TestCoordinatorGitLostAcknowledgementRecoversByReadWithoutPushReplay(t *testing.T) {
	s, p, h, ctx, out, provider := coordinatorDeploymentFixture(t, false)
	row := coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out)
	// The common deployment fixture owns a real bare remote; this flag simulates
	// losing the response after that exact mutation, not before admission.
	remote := provider.git
	remote.loseACK.Store(true)
	next, err := s.continueCoordinatorGitPublication(ctx, p, row)
	if err != nil || next.Delivery.GitPublication == nil || next.Delivery.GitPublication.Status != "unknown" || remote.pushes.Load() != 1 {
		t.Fatal(next, err, remote.pushes.Load())
	}
	reopened, _ := coordinatorDeliveryReopen(t, s)
	reopened.runner = s.runner
	reopened.SetPreviewRuntime(s.PreviewRuntime())
	reopened.SetCoordinatorDeploymentAuthorizer(s.coordinatorDeploymentAuthorize)
	for range 2 {
		next, err = reopened.continueCoordinatorGitPublication(ctx, p, next)
		if err != nil || !coordinatorGitPublished(next) || remote.pushes.Load() != 1 {
			t.Fatal("exact remote was not recovered without replay", next, err, remote.pushes.Load())
		}
	}
	a, b, c := provider.counts()
	if a+b+c != 0 {
		t.Fatal("source publication implied provider mutation", a, b, c)
	}
	if remote.observations.Load() != 2 {
		t.Fatal("exact remote was not freshly observed", remote.observations.Load())
	}
	// A later remote branch move invalidates current publication; it does not
	// authorize another push or rewrite the checked local source.
	parent, err := s.journeyGit(ctx, p.Path, "rev-parse", next.Delivery.Commit+"^")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = RunCommand(ctx, "git", []string{"--git-dir=" + remote.bare, "update-ref", "refs/heads/main", strings.TrimSpace(parent)}, ""); err != nil {
		t.Fatal(err)
	}
	next, err = reopened.continueCoordinatorGitPublication(ctx, p, next)
	if err != nil || next.Delivery.GitPublication.Status != "unknown" || remote.pushes.Load() != 1 {
		t.Fatal("moved remote silently repushed", next, err)
	}
}

func TestCoordinatorGitReservedBeforeCrashDoesNotReplayOrAdoptOtherBranch(t *testing.T) {
	s, p, h, ctx, out, provider := coordinatorDeploymentFixture(t, false)
	row := coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out)
	copy := *row.Delivery
	row.Delivery = &copy
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row.Delivery.GitPublication = &CoordinatorGitAction{ActionID: row.ID + "-git", Status: "pending", Repository: row.DeploymentPlan.Repository, Branch: row.DeploymentPlan.Branch, Commit: row.Delivery.Commit, StartedAt: now, UpdatedAt: now}
	row, err := s.persistCoordinatorGitPublication(ctx, p, row, "pending", "Controlled process loss before network invocation.")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reopened, _ := coordinatorDeliveryReopen(t, s)
		reopened.runner = s.runner
		reopened.SetPreviewRuntime(s.PreviewRuntime())
		reopened.SetCoordinatorDeploymentAuthorizer(s.coordinatorDeploymentAuthorize)
		row, err = reopened.continueCoordinatorGitPublication(ctx, p, row)
		if err != nil || row.Delivery.GitPublication.Status != "unknown" || provider.git.pushes.Load() != 0 {
			t.Fatal("reserved push replayed", row, err)
		}
	}
	a, b, c := provider.counts()
	if a+b+c != 0 {
		t.Fatal("unpublished source admitted provider", a, b, c)
	}
}

func TestCoordinatorGitFreshAuthorityAndSourceGuardPreventPublication(t *testing.T) {
	for _, drift := range []string{"authority-after-reservation", "dirty-source", "changed-origin", "missing-consent"} {
		t.Run(drift, func(t *testing.T) {
			s, p, h, ctx, out, provider := coordinatorDeploymentFixture(t, false)
			row := coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out)
			switch drift {
			case "authority-after-reservation":
				original := s.coordinatorDeploymentAuthorize
				s.SetCoordinatorDeploymentAuthorizer(func(ctx context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
					s.mu.RLock()
					rows, err := s.readContinuations(p, row.Actor.ID)
					s.mu.RUnlock()
					if err != nil {
						return management.ExecutionAuthority{}, err
					}
					if len(rows.Rows) > 0 && rows.Rows[0].Delivery != nil && rows.Rows[0].Delivery.GitPublication != nil {
						return management.ExecutionAuthority{}, errors.New("revoked after durable reservation")
					}
					return original(ctx, ref, allocate)
				})
			case "dirty-source":
				if err := os.WriteFile(filepath.Join(p.Path, "result.txt"), []byte("unreviewed source"), 0600); err != nil {
					t.Fatal(err)
				}
			case "changed-origin":
				if _, err := s.journeyGit(ctx, p.Path, "remote", "set-url", "origin", "https://github.com/foreign/project.git"); err != nil {
					t.Fatal(err)
				}
			case "missing-consent":
				plan := *row.DeploymentPlan
				plan.AcknowledgeRepositoryPush = false
				row.DeploymentPlan = &plan
			}
			if _, err := s.continueCoordinatorGitPublication(ctx, p, row); err == nil {
				t.Fatal("changed authority/source accepted")
			}
			if provider.git.pushes.Load() != 0 || provider.git.observations.Load() != 0 {
				t.Fatal("refused authority reached transport")
			}
			a, b, c := provider.counts()
			if a+b+c != 0 {
				t.Fatal("refused source admitted provider")
			}
		})
	}
}

func TestCoordinatorGitReceiptRequiresExactReviewedDestination(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := coordinatorContinuationRecord{CoordinatorContinuation: CoordinatorContinuation{ID: "continue-fixture", Delivery: &CoordinatorDelivery{Status: "verified", Commit: strings.Repeat("a", 40), GitPublication: &CoordinatorGitAction{ActionID: "continue-fixture-git", Status: "verified", Repository: "https://example.com/repository.git", Branch: "main", Commit: strings.Repeat("a", 40), StartedAt: now, UpdatedAt: now}}}, DeploymentPlan: &CoordinatorDeploymentPlan{Repository: "https://example.com/repository.git", Branch: "main", AcknowledgeRepositoryPush: true}}
	if !coordinatorGitPublished(row) {
		t.Fatal("valid exact fixture rejected")
	}
	for _, name := range []string{"action", "commit", "branch", "repository", "consent", "status", "timestamp"} {
		t.Run(name, func(t *testing.T) {
			next := row
			d := *row.Delivery
			a := *d.GitPublication
			plan := *row.DeploymentPlan
			next.Delivery = &d
			d.GitPublication = &a
			next.DeploymentPlan = &plan
			switch name {
			case "action":
				a.ActionID = "foreign-git"
			case "commit":
				a.Commit = strings.Repeat("b", 40)
			case "branch":
				a.Branch = "other"
			case "repository":
				a.Repository = "https://example.com/other.git"
			case "consent":
				plan.AcknowledgeRepositoryPush = false
			case "status":
				a.Status = "unknown"
			case "timestamp":
				a.UpdatedAt = "invalid"
			}
			if coordinatorGitPublished(next) {
				t.Fatal("foreign or incomplete publication accepted")
			}
		})
	}
}
