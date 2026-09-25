//go:build !windows

package main

const caseInsensitivePaths = false

// longPath does nothing off Windows: there are no short names to expand.
// canon's existence check (canon.go) runs here too (carried RR2-M1).
func longPath(p string) (string, error) { return p, nil }
