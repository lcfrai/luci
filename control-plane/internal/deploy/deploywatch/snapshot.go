package deploywatch

import (
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

// The values of Snapshot.ReleasesState. The lens's sentence for each is task
// 08's; the watcher only ever says which one is true.
const (
	ReleasesOK             = "ok"
	ReleasesNoHub          = "no_hub"           // no Hub is configured at all
	ReleasesNoHubProject   = "no_hub_project"   // the Hub answered: this repository is not one of its projects
	ReleasesHubKeySpelling = "hub_key_spelling" // the Hub refused the key itself
	ReleasesSignedOut      = "signed_out"       // the Hub will answer once the user signs in
	ReleasesUnavailable    = "unavailable"      // a configured Hub did not answer
)

// Snapshot is the newest reading for one local project. Deploy data is held
// only here, in memory: the watcher persists nothing (plan 7P-13).
type Snapshot struct {
	ProjectID    string
	Binding      binding.Binding
	Environments []deploy.Environment
	// BranchEnvs is git branch -> the target's environment name for it, for the
	// wanted branches only. Absent = not resolved yet. The name may match no
	// row: a branch never deployed still has one. Joined to Environments with
	// SameEnvironment, never by comparing the branch with a row's Name or
	// Branch (review C2).
	BranchEnvs map[string]string
	// BranchEnvErrs is git branch -> Panacea's refusal of EnvironmentFor for
	// it: KindNotFound, KindRejected, KindForbidden or KindConflict (carried
	// b02-2 M-5). A branch is in BranchEnvs or here, never both. It does not
	// stale the snapshot: the refusal is about one name, not about Panacea.
	// The three kinds that are not about one name are never here:
	// KindUnavailable and KindUnauthorized are the failure rule's, and
	// KindNotConnected ends the Panacea phase (no credential is not a fact
	// about this branch).
	BranchEnvErrs map[string]*deploy.Error
	Latest        *deploy.Deployment // nil = never deployed, or not yet read
	Deployments   []deploy.Deployment
	DeploymentsAt time.Time
	Runs          map[string]deploy.RunProgress // tracked runs only
	Scans         map[string]deploy.ScanReport  // by deployment id; newest deployment per environment only
	Releases      []Release                     // newest reservedAt first, at most 200: the first page only
	ReleasesMore  bool                          // that page's next was non-null (review M11)
	ReleasesAt    time.Time                     // the last successful Releases read for this binding
	// PinsByRun is deployment id -> the confirmed run release whose pipelineRun
	// is that id, from a targeted read. A nil value is the Hub's answer "none";
	// an absent id is "not read yet". Safe to cache for the binding's life only
	// because of the settle rule (SettledBy).
	PinsByRun     map[string]*Release
	ReleasesState string
	OwnerControls string // "yes" | "no" | "unknown" (7P-9)
	// FetchedAt is the end of the last poll in which every Panacea read for
	// this project succeeded.
	FetchedAt time.Time
	// Stale means the data is FetchedAt old and not current: the last poll
	// failed, or Panacea refused Environments. A budget stop never sets it.
	Stale     bool
	LastError *deploy.Error
	Version   uint64
}

// SettledBy reports whether d's release, if it has one, was final by at.
//
// pinSettle is the longest ExpiresAt - ReservedAt in s.Releases plus five
// minutes of skew, and pinSettleDefault when the page gives no window. Both
// timestamps are RFC 3339 strings on the Hub's wire (carried b09-2 M-C), so a
// row whose pair does not parse, or whose ExpiresAt is not after its
// ReservedAt, contributes nothing. Every failure is toward UNSETTLED: waiting
// one cycle longer costs a "pin not loaded"; deciding too early caches a false
// nil for the binding's life.
func (s Snapshot) SettledBy(d deploy.Deployment, at time.Time) bool {
	if d.CreatedAt.IsZero() || at.IsZero() {
		return false
	}
	return d.CreatedAt.Add(s.pinSettle()).Before(at)
}

func (s Snapshot) pinSettle() time.Duration {
	longest := time.Duration(0)
	for _, r := range s.Releases {
		reserved, err := time.Parse(time.RFC3339, r.ReservedAt)
		if err != nil {
			continue
		}
		expires, err := time.Parse(time.RFC3339, r.ExpiresAt)
		if err != nil {
			continue
		}
		if w := expires.Sub(reserved); w > longest {
			longest = w
		}
	}
	if longest <= 0 {
		return pinSettleDefault
	}
	return longest + clockSkew
}
