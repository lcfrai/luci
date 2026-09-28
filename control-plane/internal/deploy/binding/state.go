package binding

import "github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"

type State string

const (
	Bound        State = "bound"
	NoOrigin     State = "no_origin"     // the local repository has no origin remote
	UnknownHost  State = "unknown_host"  // origin is neither GitHub nor Azure DevOps
	NoMatch      State = "no_match"      // no project this key can see deploys this repository
	Ambiguous    State = "ambiguous"     // several do; the user chooses once
	NoRepoInfo   State = "no_repo_info"  // the target reported no repoUrl for any project
	NotConnected State = "not_connected" // connection.Manager has no working credential
	Unavailable  State = "unavailable"   // the target could not be reached; the last binding is kept
)

// The sentences, exactly. They are constants rather than a literal at each use
// because the lens, the readiness surface and the tests must all say the same
// thing, and three copies of a sentence is three sentences.
const (
	MessageNoOrigin    = "This repository has no origin remote, so there is nothing for Panacea to build."
	MessageUnknownHost = "Panacea builds from GitHub or Azure DevOps; this repository's origin is neither."
	MessageNoMatch     = "No Panacea project your key can see deploys this repository. A key restricted to " +
		"one project sees only that project; otherwise, create one in the Panacea portal, then come back."
	MessageAmbiguous    = "Several Panacea projects deploy this repository. Choose the one this cockpit should use."
	MessageNoRepoInfo   = "Panacea did not say which repository each project deploys. Choose the one for this repository."
	MessageNotConnected = "Connect to Panacea to see this project's deployments."
	MessageUnavailable  = "Panacea could not be reached; showing the last binding."
	MessageHubDown      = "The Hub could not be reached; showing the last Hub answer for this repository."
)

// The values of Binding.HubMiss.
//
// "" means one of three things, and the binding alone does not tell them apart,
// on purpose: HubProjectID is set; or no Hub is configured; or the Hub has
// never answered for this key. The first is told by HubProjectID != "". The
// other two are told by the WATCHER (task 07), which knows whether a Hub is
// configured and reports releasesState "no_hub" or "unavailable" - the binding
// cannot know, and a fourth value here would be a guess. Task 09's readiness
// therefore takes that cause from the view's releasesState, not from HubMiss.
//
// MissSignedOut exists because that one cause is NOT inferable from the
// watcher's side: a configured, reachable Hub that refuses for want of a
// session looks exactly like an outage, and "The Hub is not answering" sends
// the user to wait instead of to sign in (carried b05-2 N-2).
const (
	MissNotLinked   = "not_linked"   // the Hub answered: no project it lets this caller read has this key
	MissKeySpelling = "key_spelling" // the key is in no form the Hub accepts, so no Hub project can hold it
	MissSignedOut   = "signed_out"   // the Hub refused for want of a session; the user signs in
)

// Binding is one local project's answer. It is recomputed, never persisted,
// except for a user's choice between candidates (choices.go).
type Binding struct {
	ProjectID    string            `json:"projectId"`
	RepoKey      string            `json:"repoKey"`      // "" in NoOrigin and UnknownHost
	HubProjectID string            `json:"hubProjectId"` // "" when the repository is not a Hub project
	HubMiss      string            `json:"-"`            // never on a wire; TestHubKeyIsRepoKeyVerbatim pins it
	Panacea      deploy.ProjectRef `json:"panaceaProject"`
	State        State             `json:"state"`
	ChosenBy     string            `json:"chosenBy"`   // "auto" | "local" | ""
	Candidates   []deploy.Project  `json:"candidates"` // Ambiguous and NoRepoInfo only
	Message      string            `json:"message"`    // one sentence for the lens, for every State but Bound
}
