// Package hubapi is the Hub's wire contract in Go: the error codes, the key and
// id grammar, the canonical encoding, and one struct per JSON object
// (docs/superpowers/contracts/2026-09-23-hub-contract.md §3, §4, §7).
//
// Both sides import it: the Hub server to write, and the control plane's
// hubclient to read and to re-check digests. It imports the standard library and
// internal/httpx only (plan correction A9); the boundary test (task 09,
// control-plane/internal/hub/boundary_test.go) holds it to that. There is no TypeScript twin, and there must never be one: the desktop
// never computes a digest (provenance contract P5).
package hubapi
