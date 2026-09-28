package binding_test

import (
	"context"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

func resolve(t *testing.T, r *binding.Resolver, id string) binding.Binding {
	t.Helper()
	b, err := r.Resolve(context.Background(), id)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", id, err)
	}
	return b
}

// The mutant: comparing origin URLs as strings, so the SSH form of a repository
// never matches the HTTPS form Panacea reports.
func TestSingleMatchBindsSilently(t *testing.T) {
	h := newHarness(t)
	h.local("p-aaa1", "git@github.com:lcfrai/panaceaapp.git")
	h.panaceaProject("engagement-portal", "https://github.com/lcfrai/panaceaapp")
	b := resolve(t, h.build(t), "p-aaa1")

	if b.State != binding.Bound {
		t.Fatalf("State = %q, want bound (message %q)", b.State, b.Message)
	}
	if b.Panacea != "engagement-portal" {
		t.Errorf("Panacea = %q, want engagement-portal", b.Panacea)
	}
	if b.ChosenBy != "auto" {
		t.Errorf("ChosenBy = %q, want auto", b.ChosenBy)
	}
	if b.RepoKey != "github.com/lcfrai/panaceaapp" {
		t.Errorf("RepoKey = %q", b.RepoKey)
	}
	if b.Message != "" {
		t.Errorf("Bound carries a message: %q", b.Message)
	}
}

// The mutant: a GitHub-only matcher, against OD-4's two hosts.
func TestADOAndGitHubBothMatch(t *testing.T) {
	h := newHarness(t)
	h.local("p-ado1", "https://dev.azure.com/org/proj/_git/repo")
	h.local("p-gh1", "https://github.com/lcfrai/panaceaapp")
	h.panaceaProject("ado-one", "https://org.visualstudio.com/proj/_git/repo")
	h.panaceaProject("gh-one", "git@github.com:lcfrai/panaceaapp.git")
	r := h.build(t)

	if b := resolve(t, r, "p-ado1"); b.State != binding.Bound || b.Panacea != "ado-one" {
		t.Errorf("ADO: State %q Panacea %q, want bound/ado-one", b.State, b.Panacea)
	}
	if b := resolve(t, r, "p-gh1"); b.State != binding.Bound || b.Panacea != "gh-one" {
		t.Errorf("GitHub: State %q Panacea %q, want bound/gh-one", b.State, b.Panacea)
	}
}

// The mutant: an empty-handed NoMatch when Panacea simply withholds repoUrl.
func TestNoRepoInfoOffersAll(t *testing.T) {
	h := newHarness(t)
	h.local("p-aaa1", "https://github.com/lcfrai/panaceaapp")
	h.panaceaProject("one", "")
	h.panaceaProject("two", "")
	b := resolve(t, h.build(t), "p-aaa1")

	if b.State != binding.NoRepoInfo {
		t.Fatalf("State = %q, want no_repo_info", b.State)
	}
	if len(b.Candidates) != 2 {
		t.Fatalf("Candidates = %d, want 2", len(b.Candidates))
	}
	if b.Message != binding.MessageNoRepoInfo {
		t.Errorf("Message = %q", b.Message)
	}
}

// The mutant: a sentence that tells the holder of a one-project key that the
// project does not exist (review I5).
func TestNoMatchUnderARestrictedKey(t *testing.T) {
	h := newHarness(t)
	h.local("p-other", "https://github.com/lcfrai/other-repo")
	h.panaceaProject("engagement-portal", "https://github.com/lcfrai/panaceaapp")
	b := resolve(t, h.build(t), "p-other")

	if b.State != binding.NoMatch {
		t.Fatalf("State = %q, want no_match", b.State)
	}
	want := "No Panacea project your key can see deploys this repository. A key restricted to " +
		"one project sees only that project; otherwise, create one in the Panacea portal, then come back."
	if b.Message != want {
		t.Errorf("Message =\n%q\nwant\n%q", b.Message, want)
	}
	if len(b.Candidates) != 0 {
		t.Errorf("NoMatch offers %d candidates; it offers none", len(b.Candidates))
	}
}
