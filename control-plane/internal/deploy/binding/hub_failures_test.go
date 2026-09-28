package binding_test

import (
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

// The mutants: Ship disabled by a Hub blip the Ship path would catch anyway; a
// blip resetting HubMiss so the lens says "not a shared project" for one cycle;
// and a FIRST-read failure reported as "not a shared project" about a
// repository that may well be one.
func TestHubDownKeepsPanaceaBinding(t *testing.T) {
	h := newHarness(t).withHub(acceptAll)
	h.local("p-aaa1", ghOrigin)
	h.panaceaProject("portal", ghOrigin)
	h.hub.byKey["github.com/lcfrai/panaceaapp"] = "pj-1"
	r := h.build(t)

	if b := resolve(t, r, "p-aaa1"); b.HubProjectID != "pj-1" {
		t.Fatalf("first read: HubProjectID = %q", b.HubProjectID)
	}
	h.hub.fail(errHubDown)
	b := resolve(t, r, "p-aaa1")
	if b.State != binding.Bound || b.Panacea != "portal" {
		t.Errorf("a Hub blip unbound Panacea: %q %q", b.State, b.Panacea)
	}
	if b.HubProjectID != "pj-1" {
		t.Errorf("HubProjectID = %q; the last good answer is kept", b.HubProjectID)
	}
	if !strings.Contains(b.Message, "Hub") {
		t.Errorf("Message = %q; it says the Hub was unreachable", b.Message)
	}

	// A key whose last answer was key_spelling keeps it through a blip.
	k := newHarness(t).withHub(func(string) bool { return false })
	k.local("p-aaa1", ghOrigin)
	k.panaceaProject("portal", ghOrigin)
	kr := k.build(t)
	if b := resolve(t, kr, "p-aaa1"); b.HubMiss != binding.MissKeySpelling {
		t.Fatalf("seed: HubMiss = %q", b.HubMiss)
	}
	k.hub.fail(errHubDown)
	if b := resolve(t, kr, "p-aaa1"); b.HubMiss != binding.MissKeySpelling {
		t.Errorf("through a blip: HubMiss = %q, want key_spelling", b.HubMiss)
	}

	// A key whose FIRST read fails reads "", never not_linked.
	f := newHarness(t).withHub(acceptAll)
	f.local("p-aaa1", ghOrigin)
	f.panaceaProject("portal", ghOrigin)
	f.hub.fail(errHubDown)
	if b := resolve(t, f.build(t), "p-aaa1"); b.HubMiss != "" || b.HubProjectID != "" {
		t.Errorf("first read failed: HubMiss %q id %q, want \"\" and \"\"", b.HubMiss, b.HubProjectID)
	}
}

// Carried b05-2 N-1. The mutant: every Hub lookup error is a blip, so a key the
// Hub REFUSED reads as "the Hub is not answering" forever and key_spelling is
// never set against the real Hub.
func TestHubRefusedKeyIsAnAnswerNotABlip(t *testing.T) {
	h := newHarness(t).withHub(func(string) bool { return false })
	h.local("p-aaa1", ghOrigin)
	h.panaceaProject("portal", ghOrigin)
	h.hub.fail(errHubBadRepo)

	b := resolve(t, h.build(t), "p-aaa1")
	if b.HubMiss != binding.MissKeySpelling {
		t.Errorf("HubMiss = %q, want key_spelling", b.HubMiss)
	}
	if b.HubProjectID != "" {
		t.Errorf("HubProjectID = %q, want \"\"", b.HubProjectID)
	}
	if strings.Contains(b.Message, "Hub") {
		t.Errorf("Message = %q; a refused key is an answer, not an outage", b.Message)
	}
}

// Carried b05-2 N-2. The mutant: a signed-out first read reported as "The Hub
// is not answering" instead of "Sign in first."
func TestHubSignedOutIsCarriedInTheBinding(t *testing.T) {
	h := newHarness(t).withHub(acceptAll)
	h.local("p-aaa1", ghOrigin)
	h.panaceaProject("portal", ghOrigin)
	h.hub.fail(errHubSignedIn)

	b := resolve(t, h.build(t), "p-aaa1")
	if b.HubMiss != binding.MissSignedOut {
		t.Errorf("HubMiss = %q, want signed_out", b.HubMiss)
	}
	if b.State != binding.Bound {
		t.Errorf("a signed-out Hub unbound Panacea: State %q", b.State)
	}
}
