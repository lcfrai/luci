// Package ids mints identifiers and secrets. It exists so there is exactly one
// implementation: docs/FUNCTIONAL-SPEC.md known defect 2 is a home-rolled uid()
// that collides on fast successive calls, and the fix is not a better algorithm
// per call site, it is one call site.
//
// Prefixes, fixed across the product: "s-" session, "p-" project, "b-" branch,
// "w-" worktree, "m-" machine — except the local machine, whose id is the
// literal "local".
package ids

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// New returns prefix + "-" + 8 random lowercase hex characters, e.g.
// "s-3f2a91c0". 32 bits of entropy is ample for the tens-to-hundreds of
// sessions a laptop holds and keeps the id short enough to read out loud.
func New(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("ids: crypto/rand failed: " + err.Error())
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// Token returns a 256-bit URL-safe secret used as a sandbox's /attach bearer
// token. It is never logged and never returned over the REST surface.
func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("ids: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
