package binding_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

const ghOrigin = "https://github.com/lcfrai/panaceaapp"

func twoMatches(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.local("p-aaa1", ghOrigin)
	h.panaceaProject("portal-dev", ghOrigin)
	h.panaceaProject("portal-uat", "git@github.com:lcfrai/panaceaapp.git")
	return h
}

// The mutant: a choice that does not survive a restart, so the question is
// asked again every time the cockpit opens.
func TestAmbiguousAsksOnce(t *testing.T) {
	h := twoMatches(t)
	r := h.build(t)

	b := resolve(t, r, "p-aaa1")
	if b.State != binding.Ambiguous || len(b.Candidates) != 2 {
		t.Fatalf("State %q with %d candidates, want ambiguous with 2", b.State, len(b.Candidates))
	}
	if b.Message != binding.MessageAmbiguous {
		t.Errorf("Message = %q", b.Message)
	}

	chosen, err := r.Choose(context.Background(), "p-aaa1", "portal-uat")
	if err != nil {
		t.Fatalf("Choose: %v", err)
	}
	if chosen.State != binding.Bound || chosen.Panacea != "portal-uat" || chosen.ChosenBy != "local" {
		t.Fatalf("after Choose: %q %q %q", chosen.State, chosen.Panacea, chosen.ChosenBy)
	}

	again := resolve(t, h.build(t), "p-aaa1") // a fresh Resolver over the same dir
	if again.State != binding.Bound || again.Panacea != "portal-uat" || again.ChosenBy != "local" {
		t.Fatalf("after restart: %q %q %q", again.State, again.Panacea, again.ChosenBy)
	}
}

// The mutant: applying a choice made against dev to prod.
func TestChoiceScopedToOrigin(t *testing.T) {
	h := twoMatches(t)
	h.origin = "https://dev.panacea.kpmg"
	r := h.build(t)
	if _, err := r.Choose(context.Background(), "p-aaa1", "portal-uat"); err != nil {
		t.Fatalf("Choose: %v", err)
	}

	h.origin = "https://panacea.kpmg"
	if b := resolve(t, h.build(t), "p-aaa1"); b.State != binding.Ambiguous {
		t.Fatalf("against another Panacea: State %q Panacea %q, want ambiguous", b.State, b.Panacea)
	}
}

// The mutant: staying bound to a project the token can no longer see.
func TestStaleChoiceDropped(t *testing.T) {
	h := twoMatches(t)
	if _, err := h.build(t).Choose(context.Background(), "p-aaa1", "portal-uat"); err != nil {
		t.Fatalf("Choose: %v", err)
	}

	h2 := newHarness(t)
	h2.dir = h.dir
	h2.local("p-aaa1", ghOrigin)
	h2.panaceaProject("portal-dev", ghOrigin) // portal-uat is gone
	b := resolve(t, h2.build(t), "p-aaa1")
	if b.State != binding.Bound || b.Panacea != "portal-dev" || b.ChosenBy != "auto" {
		t.Fatalf("stale choice: %q %q %q, want bound/portal-dev/auto", b.State, b.Panacea, b.ChosenBy)
	}
}

// The mutant: a free-text binding that points Ship at any project name.
func TestChooseOutsideCandidates(t *testing.T) {
	h := twoMatches(t)
	r := h.build(t)
	if _, err := r.Resolve(context.Background(), "p-aaa1"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := r.Choose(context.Background(), "p-aaa1", "portal-anything"); deploy.KindOf(err) != deploy.KindRejected {
		t.Fatalf("Choose outside the candidates: err = %v, want rejected", err)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "bindings.json")); !os.IsNotExist(err) {
		t.Errorf("a rejected choice wrote bindings.json (stat err %v)", err)
	}
}

// The mutant: a crash at start on a file the user can simply re-answer.
func TestUnreadableChoicesFile(t *testing.T) {
	h := twoMatches(t)
	if err := os.MkdirAll(h.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "bindings.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b := resolve(t, h.build(t), "p-aaa1"); b.State != binding.Ambiguous {
		t.Fatalf("State = %q, want ambiguous over an empty choice set", b.State)
	}
	names, _ := filepath.Glob(filepath.Join(h.dir, "bindings.json.unreadable-*"))
	if len(names) != 1 {
		t.Fatalf("renamed files = %v, want exactly one", names)
	}
	if data, _ := os.ReadFile(names[0]); !strings.Contains(string(data), "not json") {
		t.Errorf("the renamed file lost the original bytes")
	}
}
