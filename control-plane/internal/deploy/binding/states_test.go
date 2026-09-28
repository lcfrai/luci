package binding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

// The three states that are decided before Panacea is ever called, each with
// the sentence the lens shows. The mutant: a project with no origin, or an
// origin on a host neither git host owns, reported as "no Panacea project
// deploys this repository" - which sends the user to the portal to create one
// that could never match.
func TestLocalOnlyStatesAndSentences(t *testing.T) {
	h := newHarness(t)
	h.local("p-none", "")
	h.local("p-weird", "https://gitlab.example.com/team/thing.git")
	h.panaceaProject("engagement-portal", "https://github.com/lcfrai/panaceaapp")
	r := h.build(t)

	none := resolve(t, r, "p-none")
	if none.State != binding.NoOrigin || none.Message != binding.MessageNoOrigin {
		t.Errorf("no origin: State %q Message %q", none.State, none.Message)
	}
	if none.RepoKey != "" {
		t.Errorf("no origin carries RepoKey %q", none.RepoKey)
	}
	weird := resolve(t, r, "p-weird")
	if weird.State != binding.UnknownHost || weird.Message != binding.MessageUnknownHost {
		t.Errorf("unknown host: State %q Message %q", weird.State, weird.Message)
	}
	if weird.RepoKey != "" {
		t.Errorf("unknown host carries RepoKey %q", weird.RepoKey)
	}
}

// The mutant: the lens flipping to "not bound" during a Panacea blip.
func TestPanaceaDownKeepsLastBinding(t *testing.T) {
	h := newHarness(t)
	h.local("p-aaa1", "https://github.com/lcfrai/panaceaapp")
	h.panaceaProject("engagement-portal", "https://github.com/lcfrai/panaceaapp")
	r := h.build(t)

	if b := resolve(t, r, "p-aaa1"); b.State != binding.Bound {
		t.Fatalf("first resolve: State %q, want bound", b.State)
	}
	h.fake.FailNext("Projects", &deploy.Error{Kind: deploy.KindUnavailable, Op: "projects"})

	b, err := r.Resolve(context.Background(), "p-aaa1")
	if err == nil {
		t.Fatal("Resolve returned no error while Panacea was down")
	}
	if deploy.KindOf(err) != deploy.KindUnavailable {
		t.Errorf("KindOf(err) = %q", deploy.KindOf(err))
	}
	if b.State != binding.Unavailable {
		t.Errorf("State = %q, want unavailable", b.State)
	}
	if b.Panacea != "engagement-portal" {
		t.Errorf("Panacea = %q; the last good binding is kept", b.Panacea)
	}
	if b.Message != binding.MessageUnavailable {
		t.Errorf("Message = %q", b.Message)
	}
}

// The mutant: a cockpit with no credential reported as "no Panacea project
// deploys this repository", which is a statement about Panacea nobody made.
func TestNotConnectedIsItsOwnState(t *testing.T) {
	h := newHarness(t)
	h.local("p-aaa1", "https://github.com/lcfrai/panaceaapp")
	h.fake.FailNext("Projects", deploy.ErrNotConnected)
	r := h.build(t)

	b, err := r.Resolve(context.Background(), "p-aaa1")
	if !errors.Is(err, deploy.ErrNotConnected) && deploy.KindOf(err) != deploy.KindNotConnected {
		t.Fatalf("err = %v, want not_connected", err)
	}
	if b.State != binding.NotConnected || b.Message != binding.MessageNotConnected {
		t.Errorf("State %q Message %q", b.State, b.Message)
	}
	if b.RepoKey != "github.com/lcfrai/panaceaapp" {
		t.Errorf("RepoKey = %q; the local side is still known", b.RepoKey)
	}
}

// The mutant: an unknown local project id answered with an empty binding
// instead of the sentinel the 404 is written from.
func TestUnknownLocalProject(t *testing.T) {
	h := newHarness(t)
	h.local("p-aaa1", "https://github.com/lcfrai/panaceaapp")
	if _, err := h.build(t).Resolve(context.Background(), "p-nope"); !errors.Is(err, binding.ErrUnknownProject) {
		t.Fatalf("err = %v, want ErrUnknownProject", err)
	}
}
