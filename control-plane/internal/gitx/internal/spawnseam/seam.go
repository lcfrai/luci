// Package spawnseam is gitx.Exec's process seam. It is internal to gitx, so production code
// outside gitx cannot reach it, and gitxtest can.
package spawnseam

// Spawn is a child's final argv, working directory and environment.
type Spawn struct {
	Argv []string
	Dir  string
	Env  []string
}

// Set is filled by package gitx's init: it installs f as e's spawn observer (nil removes it).
// e is a *gitx.Exec; any other type panics.
var Set func(e any, f func(Spawn))
