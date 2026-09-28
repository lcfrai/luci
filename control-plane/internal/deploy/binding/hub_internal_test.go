package binding

import "testing"

// missCause chooses a SENTENCE, never an answer: the lookup is always made
// first. The mutant: a pattern of phase 7's own in place of the Hub's rule -
// a fourth canonicaliser that drifts the day the Hub widens what it accepts.
// The rule is injected (Config.HubKeyAccepted, main.go's hubapi.HostOfRepoKey),
// so this test pins the ROUTING, and the rule's own cases belong to the Hub.
func TestMissCauseRoutesOnTheHubsRule(t *testing.T) {
	refuseOld := func(k string) bool {
		switch k {
		case "azure-devops/kpmgcomau/engagement portal/portal", "github.com/lcfrai/panaceaapp":
			return true
		}
		return false // the old .visualstudio.com form; a GitHub owner with "_"
	}
	cases := []struct{ key, want string }{
		{"azure-devops/kpmgcomau/engagement portal/portal", MissNotLinked},
		{"kpmgcomau.visualstudio.com/engagement portal/portal", MissKeySpelling},
		{"github.com/lcfrai/panaceaapp", MissNotLinked},
		{"github.com/dmackie1_kpmg/portal", MissKeySpelling},
	}
	for _, c := range cases {
		if got := missCause(refuseOld, c.key); got != c.want {
			t.Errorf("missCause(%q) = %q, want %q", c.key, got, c.want)
		}
	}

	// With no rule wired - no Hub, or a Hub whose rule main.go could not supply
	// - phase 7 never claims a key is misspelt on its own authority.
	if got := missCause(nil, "kpmgcomau.visualstudio.com/engagement portal/portal"); got != MissNotLinked {
		t.Errorf("missCause(nil, ...) = %q, want not_linked", got)
	}
}
