package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

// Workers keep their own lease so an orphan after a gateway crash still blocks
// backup. Every Linux root holds the lease from initial bootstrap, so commissioning
// cannot race an older unbound gateway. It is released after gateway shutdown.
func acquireManagedStateLease(cfg Config) (*os.File, error) {
	root := cfg.StateDir
	if root == "" {
		root = filepath.Dir(filepath.Dir(cfg.SessionsPath))
	}
	_, markerErr := os.Lstat(filepath.Join(root, "control-plane", "managed-team.json"))
	managed := cfg.ManagedTeamID != "" || markerErr == nil
	if markerErr != nil && !os.IsNotExist(markerErr) {
		return nil, fmt.Errorf("inspect managed host binding: %w", markerErr)
	}
	if !managed && runtime.GOOS != "linux" {
		return nil, nil
	}
	return statelease.Acquire(root, product.Version)
}
