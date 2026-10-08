package main

import (
	"context"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type recoverCreatedBackend struct {
	*recoveryBackend
	destroyed []string
}

func (b *recoverCreatedBackend) Destroy(_ context.Context, id string) error {
	b.destroyed = append(b.destroyed, id)
	b.sandboxes = nil
	return nil
}

func TestRestartThenSweepRecoversExactInterruptedCreatedChildWithoutDispatch(t *testing.T) {
	for _, arrivesLate := range []bool{false, true} {
		t.Run(map[bool]string{false: "present-at-restart", true: "arrives-after-restart"}[arrivesLate], func(t *testing.T) {
			store := bootStore(t)
			const instance = "11111111-1111-4111-8111-111111111111"
			row := registry.Session{ID: "s-11111111", SandboxID: "s-22222222", Status: registry.StatusStarting, ProjectID: "p-original", ProjectRunID: "run-original", ProjectRunOperationID: "op-original", OpeningPrompt: "original reviewed context", AttachToken: "private-token"}
			bootSeed(t, store, row)
			sb := backend.Sandbox{ID: row.SandboxID, Ref: "exact-original-container", Instance: instance, Kind: backend.KindDocker, CreatedOnly: true, State: backend.StateStarting, CreatedAt: bootClock.Add(-3 * time.Minute), Labels: map[string]string{backend.LabelManaged: "true", backend.LabelSandboxID: row.SandboxID, backend.LabelSessionID: row.ID}}
			be := &recoverCreatedBackend{recoveryBackend: &recoveryBackend{}}
			if !arrivesLate {
				be.sandboxes = []backend.Sandbox{sb}
			}
			recoverSessions(context.Background(), store, be, quietLogger())
			if len(be.destroyed) != 0 {
				t.Fatal("startup performed destructive cleanup")
			}
			be.sandboxes = []backend.Sandbox{sb}
			r := reaper.New(reaper.Config{Store: store, Backend: be, Instance: instance, Now: func() time.Time { return bootClock }, Log: quietLogger()})
			for range 2 {
				if err := r.Sweep(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			got := bootGet(t, store, row.ID)
			if got.Status != registry.StatusExited || got.ProjectRunID != row.ProjectRunID || got.ProjectRunOperationID != row.ProjectRunOperationID || got.OpeningPrompt != row.OpeningPrompt || got.SandboxID != row.SandboxID {
				t.Fatal("lost interrupted allocation identity", got)
			}
			if len(be.destroyed) != 1 || be.destroyed[0] != row.SandboxID {
				t.Fatal("did not reclaim exact owned child", be.destroyed)
			}
			if !arrivesLate && got.ExitReason != "sandbox creation was interrupted before the agent started; review this attempt's recovery before retrying" {
				t.Fatal(got.ExitReason)
			}
		})
	}
}
