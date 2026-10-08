package main

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func executionStateDirectory(cfg Config, inherited string) (string, error) {
	path := inherited
	if path == "" {
		if cfg.Worker {
			return "", errors.New("managed workers require the host execution state directory")
		}
		root := cfg.StateDir
		if root == "" {
			root = filepath.Dir(filepath.Dir(cfg.SessionsPath))
		}
		path = filepath.Join(root, "control-plane", "execution")
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("host execution state directory must be absolute and canonical")
	}
	return path, nil
}

func executionStorageConfirmationPath(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "local-storage-confirmation.json")
}

func (b *recoveringBackend) LocalStorageStatus() resourcebudget.LocalStorageStatus {
	// A read-only Settings GET must never reconnect Docker or run native I/O.
	b.mu.Lock()
	value, closed := b.current, b.closed
	b.mu.Unlock()
	if !closed {
		if configurer, ok := value.(management.ExecutionStorageConfigurer); ok {
			return configurer.LocalStorageStatus()
		}
	}
	scope := "not-required"
	if runtime.GOOS == "windows" {
		scope = "local-docker-desktop"
	}
	return resourcebudget.LocalStorageStatus{Platform: runtime.GOOS, Scope: scope, Required: runtime.GOOS == "windows", State: "unavailable", ReasonCode: "runtime_unavailable", Message: "The execution runtime has no current local-storage observation. Start Docker Desktop and wait for background observation."}
}

func (b *recoveringBackend) PrepareLocalStorageConfirmation(ctx context.Context, path string) (resourcebudget.LocalStorageConfirmation, error) {
	value, err := b.get()
	if err != nil {
		return resourcebudget.LocalStorageConfirmation{}, err
	}
	if configurer, ok := value.(management.ExecutionStorageConfigurer); ok {
		return configurer.PrepareLocalStorageConfirmation(ctx, path)
	}
	return resourcebudget.LocalStorageConfirmation{}, errors.New("execution runtime does not support local Windows storage confirmation")
}
