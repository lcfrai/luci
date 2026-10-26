//go:build !windows

package management

func newManagedModelPlatform(string) managedModelPlatform { return nil }
