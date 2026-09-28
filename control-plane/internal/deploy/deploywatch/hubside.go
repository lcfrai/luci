package deploywatch

import (
	"context"
	"time"
)

// This file is the Hub, as the watcher is allowed to know it. Verified in this
// worktree on 2026-09-28: control-plane/internal holds no hubapi and no
// hubclient package, so these are DECLARED STAND-INS, the same shape task 06
// used for the one Hub lookup a binding needs (binding.HubProjects). Their
// fields mirror the Hub contract's wire and the Hub plan's Go types, so the
// adapter written the day hubclient lands is a field-for-field copy and not a
// re-interpretation.
//
// What phase 7 deliberately does NOT know: a Hub status code, a Hub problem
// body, or the Hub's key rule. A failure arrives as an error and is classified
// by main.go, with binding.HubErrorClass - one vocabulary for "the Hub did not
// answer", not a second one here.

// PageRequest is the Hub's paging shape (Hub plan phase-h-hub-parts/task-03.md
// :207-216). A zero Limit means the Hub's own default of 50, NOT the maximum,
// which is why every query the watcher sends sets it (carried b09-2 M-C).
type PageRequest struct {
	After string
	Limit int
}

// ReleaseQuery is the subset of the Hub's release query the watcher sends.
// Kind and State are the Hub's own words, passed through, never interpreted.
type ReleaseQuery struct {
	PanaceaProject string
	PipelineRun    string
	State          string
	Kind           string
	Page           PageRequest
}

// ReleasePage is one page of releases. Next is nil when the page is the whole
// answer; a non-nil Next is what ReleasesMore reports (review M11).
type ReleasePage struct {
	Items []Release
	Next  *string
}

// Release is one reservation of a brief against a deployment. ReservedAt and
// ExpiresAt are RFC 3339 STRINGS, as the Hub's own type has them: this package
// parses them in exactly one place (pinSettle) and fails toward unsettled.
type Release struct {
	ID           string
	Kind         string // "run" | "rollback", verbatim
	State        string // "reserved" | "confirmed" | "abandoned" | "expired", verbatim
	PipelineRun  string // the deployment id the run will produce; "" until confirm
	ReservedByID string // the Hub principal who reserved it
	ReservedAt   string
	ExpiresAt    string
	// DeploymentIDs is the release's deployments[] entries. Empty means the
	// join has not been written, which is what step 7 writes.
	DeploymentIDs []string
}

// Event is one entry of the Hub's change feed (§8). Two fields, because two
// fields are what the watcher reads.
type Event struct {
	Kind      string
	ProjectID string
}

// Releases is the Hub's release surface, narrowed to the two calls the watcher
// makes. JoinDeployment's answer is discarded - the watcher writes the join and
// reads nothing back - so the port returns only the error.
type Releases interface {
	Releases(ctx context.Context, hubProjectID string, q ReleaseQuery) (ReleasePage, error)
	JoinDeployment(ctx context.Context, releaseID, deploymentID string) error
}

// Me names the signed-in Hub principal, narrowed to the id, because the id is
// the whole of what step 7 compares: the watcher joins only releases THIS user
// reserved.
type Me interface {
	MeID(ctx context.Context) (string, error)
}

// pinSettleDefault is the wait before a deployment with no release in the page
// can be read as having none: the Hub's reservation TTL (15 min) plus 5 min of
// clock skew. It stands whenever no row in the page gives a real window.
const pinSettleDefault = 20 * time.Minute

// clockSkew is added to the longest observed reservation window.
const clockSkew = 5 * time.Minute
