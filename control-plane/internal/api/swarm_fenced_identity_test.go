package api

import (
	"context"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSwarmFencedRecoveryStrengthensOnlyOriginallyUnknownContainerReference(t *testing.T) {
	for _, known := range []bool{false, true} {
		name := "originally unknown reference may be strengthened"
		if known {
			name = "recorded reference cannot be replaced"
		}
		t.Run(name, func(t *testing.T) {
			f, probe, _ := fencedTaskFixture(t)
			firstRef, learnedRef := fixtureFenceHash("original immutable container"), fixtureFenceHash("subsequently observed container")
			if known {
				if _, err := f.store.Update(f.ops[0].ChildID, func(row *registry.Session) error { row.SandboxRef = firstRef; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			intent, release, err := f.env.api.acquireSwarmFenceRecovery(context.Background(), fencedRecoveryRequest(t, f))
			if err != nil {
				t.Fatal(err)
			}
			release() // Original private intent survived; product write did not.
			if _, err := f.store.Update(f.ops[0].ChildID, func(row *registry.Session) error { row.SandboxRef = learnedRef; return nil }); err != nil {
				t.Fatal(err)
			}
			f.reopen(t)
			if known {
				fencedRecover(t, f, 409)
				ledger := assertFencedLedger(t, f, 1, "intent")
				if ledger.Fences[0].Intent != intent || len(probe.requests) != 0 {
					t.Fatal("saved immutable original was relaxed or backend mutated")
				}
				return
			}
			fencedRecover(t, f, 200)
			ledger := assertFencedLedger(t, f, 0, "retired")
			if len(probe.requests) != 1 || probe.requests[0].ExpectedOriginalRef != learnedRef || ledger.Fences[0].Intent != intent || intent.FenceRequest.ExpectedOriginalRef != "" {
				t.Fatal("retry did not strengthen backend deletion precondition while preserving exact saved intent")
			}
		})
	}
}
