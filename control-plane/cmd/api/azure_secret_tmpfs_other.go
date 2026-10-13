//go:build !linux

package main

// The selected cloud worker is Linux-only. Native Windows startup compiles the
// optional Azure route but must never materialize cloud credentials there.
func azureSecretTmpfs(string) bool { return false }
