//go:build !linux

package main

import "errors"

func azurePathDevice(string) uint64 { return 0 }
func azureMountedDevice(string, string) (uint64, error) {
	return 0, errors.New("Azure controller disk requires Linux")
}
