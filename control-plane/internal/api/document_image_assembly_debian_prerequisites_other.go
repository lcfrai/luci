//go:build !windows

package api

import "context"

// Unsupported platforms never start a helper, probe, or fallback process.
func observeDebianHostPrerequisites(context.Context, *debianProofNativeOwner, debianHostRequirements) (debianHostFacts,error) {
	return debianHostFacts{},errDebianHostFacts
}
