//go:build !linux

package main

// The only selected driver target is the pinned Linux container. A native
// Windows/other-platform invocation has no no-follow ordinary-file proof.
func readBoundedRegular(string, int) ([]byte, error) { return nil, errRefused }

func newPilotStage(string) (pilotStage, error) { return nil, errRefused }
