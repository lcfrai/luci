package api

import (
	"context"
	"net/http"
)

// Materialiser writes the brief a session inherits into its worktree BEFORE
// the harness process starts, and says what it wrote (ruling R1; provenance
// contract §2.4). Phase 4's contextwrite satisfies it; api imports nothing of
// phase 4.
//
// THE ORDERING RULE: handleSessionCreate and handleSessionRestart call it
// after the request validates (worktree, harness, argv) and before anything
// is recorded or created. On create it also comes before the conversation
// store's MkdirAll, so a refusal touches none of the handler's own disk (a
// Materialiser's partial effects are its own, task 09's); on restart it comes
// before the old sandbox's destroy (a refusal must leave the running agent
// running). Called after the process starts, the session inherits nothing,
// silently.
type Materialiser interface {
	Materialise(ctx context.Context, req MaterialiseRequest) (MaterialiseResult, error)
}

type MaterialiseRequest struct {
	WorktreeID   string
	WorktreePath string // absolute, from Deps.Worktrees.Path
	// ProjectID is local: sessionLocate's on create, the record's on restart
	// (re-located when ""; re-review R2-M2 item 4). "" is a worktree outside
	// every project, enumerated cleanly: nothing to inherit (task 09 step 4).
	// An enumeration FAILURE never arrives as "": the handler refuses it as
	// project_unknown first (review I2), unless WithoutBrief is set.
	ProjectID string
	BranchID  string // local; "" for a detached worktree
	// WithoutBrief is the person's "Start without the brief" (4-D1). The
	// Materialiser removes every managed block, or refuses when it cannot
	// (task 09 step 3), and returns ContextRef "none:declined"; it does not
	// ask the Hub.
	WithoutBrief bool
}

type MaterialiseResult struct {
	// ContextRef is stamped on the session record VERBATIM: a brief key,
	// "" when no brief exists to inherit, or "none:declined". The handler
	// never interprets it (types.ts's promise, kept on the server too).
	ContextRef string
}

// MaterialiseError is a refusal the person can act on: the handler answers
// 409 brief_not_written with Reason in details and Message as the sentence.
// Any other error from Materialise is a 500.
type MaterialiseError struct {
	Reason  string
	Message string
}

func (e *MaterialiseError) Error() string { return "api: brief not written: " + e.Reason }

// The reasons, closed. The desktop switches on them (task 20).
const (
	BriefReasonHubUnavailable = "hub_unavailable"   // the Hub did not answer; a brief may exist
	BriefReasonSignedOut      = "signed_out"        // no Hub token; a brief may exist
	BriefReasonWriteFailed    = "write_failed"      // the file could not be read or written
	BriefReasonBlockBroken    = "block_broken"      // a marker with no partner (a begin with no end, a stray or early end) or two blocks
	BriefReasonTracked        = "tracked_elsewhere" // CLAUDE.md and .claude/CLAUDE.md both tracked (P4-D)
	// BriefReasonProjectUnknown is raised by the HANDLER, never by a
	// Materialiser: sessionLocate could not enumerate the projects, or could
	// not read a project's worktrees and did not find this one elsewhere, so
	// which project's brief applies is unknown (review I2). A brief may exist.
	BriefReasonProjectUnknown = "project_unknown"
)

// RouteMounter lets a package outside api register routes through the
// recorder, so contract §10a's audit sees every pattern (the recorder is
// unexported, and a *http.ServeMux registration would escape it).
type RouteMounter interface {
	Mount(handle func(pattern string, h http.Handler))
}
