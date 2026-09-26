// Package briefref is what phase 3 needs from phase 4's provenance package (asks P4-1 and
// P4-3). It declares the interfaces; phase 4 satisfies them. Phase 3 never parses a brief key
// (contract P5): it passes the string to Resolve and copies it into git unchanged.
// It lives in task 03, not task 09, because tasks 09 (Resolver), 11 (Archiver) and 15 (Criterion)
// all import it and 11 runs in a lane parallel to 09 (review I7). No code, so no tests.
package briefref

import "context"

// Revision is one brief revision as phase 3 needs it.
type Revision struct {
	Key      string      // e.g. "brief:b-42@6", exactly as Resolve was given it
	Digest   string      // e.g. "sha256:…", the revision's digest (contract §1.2)
	Criteria []Criterion // in the revision's order
}

// Criterion is one criterion's identity. Digest is SHA-256 of the criterion's text as phase 4
// computes it (ask P4-2); evidence (task 15) matches on it, because the id is positional and
// breaks on insertion (risk R14).
type Criterion struct {
	ID     string
	Digest string
}

// Resolver answers a brief key. ok is false when no revision answers it ("missing" in the
// contract's six-state vocabulary); err is for a resolver that could not answer at all.
type Resolver interface {
	Resolve(ctx context.Context, key string) (rev Revision, ok bool, err error)
}

// Archiver archives a product branch's brief head when the branch is deleted (contract P13:
// "may archive the head; revisions stay"). Called by task 11; one way, phase 3 → phase 4.
type Archiver interface {
	ArchiveHead(ctx context.Context, projectID, branchID string) error
}
