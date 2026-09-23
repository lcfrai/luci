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

// UUID returns a random RFC 4122 version 4 UUID in the canonical lowercase
// 8-4-4-4-12 form, e.g. "3f2a91c0-7b1e-4d2a-9c4f-0e8b6a5d1c23".
//
// It has one caller: the conversation id a harness session is started with.
// `claude --session-id` requires a UUID, and the control plane MINTS that id
// and tells the CLI, because the only other way to learn it is to read the
// CLI's output, which docs/PROGRAM.md forbids. It carries no prefix for the
// same reason — the CLI rejects anything that is not a UUID.
func UUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("ids: crypto/rand failed: " + err.Error())
	}
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // the RFC 4122 variant, 10xx
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
