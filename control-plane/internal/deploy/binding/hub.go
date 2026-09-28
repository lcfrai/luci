package binding

// HubErrorClass is what a Hub lookup failure MEANS, which is the only thing
// this package is allowed to know about hubclient's errors. main.go supplies
// the classifier; a status code or a problem body read here would put a second
// copy of the Hub's wire in phase 7.
type HubErrorClass string

const (
	// HubBlip is a transport failure, a timeout or a 5xx: no answer. The
	// previous HubProjectID and the previous HubMiss are both kept.
	HubBlip HubErrorClass = "blip"

	// HubRefusedKey is the Hub ANSWERING that the key is not one it accepts -
	// route 13's 400 with field "repo". It is not an outage: HubProjectID
	// becomes "" and HubMiss becomes key_spelling, on the Hub's own authority
	// (carried b05-2 N-1).
	HubRefusedKey HubErrorClass = "refused_key"

	// HubSignedOut is a Hub that will answer once the user signs in.
	HubSignedOut HubErrorClass = "signed_out"
)

// hubKey is the ONE place the key sent to the Hub is chosen, and it returns the
// key verbatim. Phase 7 never re-spells a key: that would be a fourth
// canonicaliser beside phase 3's githost, the Hub's and Panacea's, and the
// sentence it produced could then disagree with the Hub that answered.
func hubKey(r Remote) string { return r.RepoKey }

// missCause chooses the SENTENCE for a Hub miss; it never decides the answer,
// because the lookup is always made first. accepted is the Hub's own published
// rule (hubapi.HostOfRepoKey, wired in main.go). With no rule wired, phase 7
// does not claim a key is misspelt on its own authority.
func missCause(accepted func(string) bool, repoKey string) string {
	if accepted != nil && !accepted(repoKey) {
		return MissKeySpelling
	}
	return MissNotLinked
}
