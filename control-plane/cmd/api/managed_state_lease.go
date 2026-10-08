package main

import (
	"os"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

// Every gateway and worker owns its state root before bootstrap or opening any
// stores. The lease is independent of the listening address and managed binding;
// process exit releases it so the native app can restart its own sidecar safely.
// Linux offline recovery uses the same lease to exclude live writers.
func acquireManagedStateLease(cfg Config) (*os.File, error) {
	root := cfg.StateDir
	if root == "" {
		root = filepath.Dir(filepath.Dir(cfg.SessionsPath))
	}
	return statelease.Acquire(root, product.Version)
}
