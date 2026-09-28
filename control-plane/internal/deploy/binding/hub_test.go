package binding_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

func acceptAll(string) bool { return true }

// The mutant: a check that marks every miss key_spelling, and a Hub that is not
// configured at all read as "this repository is not a shared project".
func TestHubMissNotLinked(t *testing.T) {
	h := newHarness(t).withHub(acceptAll)
	h.local("p-aaa1", ghOrigin)
	h.panaceaProject("portal", ghOrigin)
	b := resolve(t, h.build(t), "p-aaa1")
	if b.HubProjectID != "" || b.HubMiss != binding.MissNotLinked {
		t.Errorf("hub configured, no project: id %q miss %q", b.HubProjectID, b.HubMiss)
	}
	if b.State != binding.Bound {
		t.Errorf("a Hub miss unbound Panacea: State %q", b.State)
	}

	n := newHarness(t) // no Hub configured
	n.local("p-aaa1", ghOrigin)
	n.panaceaProject("portal", ghOrigin)
	if nb := resolve(t, n.build(t), "p-aaa1"); nb.HubMiss != "" {
		t.Errorf("no Hub configured: HubMiss = %q, want \"\"", nb.HubMiss)
	}
}

// The mutant: a check keyed on the HOST ("Azure DevOps -> key_spelling"), and a
// stale copy of the contract's old key regex in place of the Hub's own rule.
func TestHubMissKeySpellingComesFromTheHubsOwnRule(t *testing.T) {
	refuse := func(k string) bool { return !strings.Contains(k, "visualstudio.com") && !strings.Contains(k, "_") }
	h := newHarness(t).withHub(refuse)
	h.local("p-gh", "https://github.com/dmackie1_kpmg/portal")
	h.panaceaProject("portal", "https://github.com/dmackie1_kpmg/portal")
	if b := resolve(t, h.build(t), "p-gh"); b.HubMiss != binding.MissKeySpelling {
		t.Errorf("a GitHub owner the Hub refuses: HubMiss = %q", b.HubMiss)
	}

	ok := newHarness(t).withHub(refuse)
	ok.local("p-ado", "https://dev.azure.com/kpmgcomau/Engagement%20Portal/_git/portal")
	ok.panaceaProject("portal", "https://dev.azure.com/kpmgcomau/Engagement%20Portal/_git/portal")
	if b := resolve(t, ok.build(t), "p-ado"); b.HubMiss != binding.MissNotLinked {
		t.Errorf("an Azure DevOps key the Hub accepts: HubMiss = %q, want not_linked", b.HubMiss)
	}
}

// The mutant: phase 7 re-spelling keys itself - a fourth canonicaliser - and a
// lookup skipped when the Hub's rule refuses the key, which would never bind
// once the Hub widened its rule.
func TestHubKeyIsRepoKeyVerbatim(t *testing.T) {
	refuse := func(k string) bool { return !strings.Contains(k, "_") }
	h := newHarness(t).withHub(refuse)
	h.local("p-gh", ghOrigin)
	h.local("p-ado", "https://dev.azure.com/kpmgcomau/Engagement%20Portal/_git/portal")
	h.local("p-us", "https://github.com/dmackie1_kpmg/portal")
	r := h.build(t)
	for _, id := range []string{"p-gh", "p-ado", "p-us"} {
		resolve(t, r, id)
	}

	want := []string{
		"github.com/lcfrai/panaceaapp",
		"azure-devops/kpmgcomau/engagement portal/portal",
		"github.com/dmackie1_kpmg/portal",
	}
	got := h.hub.seen()
	if len(got) != len(want) {
		t.Fatalf("keys asked = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("key %d = %q, want %q", i, got[i], want[i])
		}
	}

	raw, err := json.Marshal(binding.Binding{HubMiss: binding.MissKeySpelling})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for k := range m {
		if strings.EqualFold(k, "hubMiss") {
			t.Fatalf("HubMiss is on the binding wire: %s", raw)
		}
	}
}
