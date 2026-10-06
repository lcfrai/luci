//go:build linux

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

func TestManagedRootAndOrphanWorkerLeaseBeforeAnyStateWrite(t *testing.T) {
	for _, kind := range []string{"initial-bootstrap", "requested-root", "bound-root", "worker"} {
		t.Run(kind, func(t *testing.T) {
			dir := privateStateDir(t)
			cfg := Config{StateDir: dir, InstancePath: filepath.Join(dir, "instance", "instance-id")}
			switch kind {
			case "requested-root":
				cfg.ManagedTeamID = "team-abcdefghijklmnopqrstuv"
			case "bound-root":
				if err := os.Mkdir(filepath.Join(dir, "control-plane"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "control-plane", "managed-team.json"), []byte("existing marker"), 0600); err != nil {
					t.Fatal(err)
				}
			case "worker":
				cfg.Worker = true
			}
			holder, err := statelease.Acquire(dir, "0.5.0")
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Close()
			if err := serve(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
				t.Fatal("server ignored held state lease")
			}
			if _, err := os.Stat(cfg.InstancePath); !os.IsNotExist(err) {
				t.Fatal("server wrote state before acquiring lease")
			}
		})
	}
}
