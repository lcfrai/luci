package reaper_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type creationObservationBackend struct {
	*fakeBackend
	observe func(backend.Sandbox) (backend.Sandbox, error)
}

func (b *creationObservationBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	sb, err := b.fakeBackend.Get(ctx, id)
	if err != nil {
		return sb, err
	}
	if b.observe != nil {
		return b.observe(sb)
	}
	return sb, nil
}

func interruptedChild() backend.Sandbox {
	sb := managed("s-created", backend.StateStarting)
	sb.CreatedOnly, sb.Endpoint = true, ""
	sb.Labels[backend.LabelSessionID] = "s-session"
	return sb
}

func TestInterruptedCreatedChildConvergesWithoutRecreatingOrLosingIdentity(t *testing.T) {
	store := newStore(t)
	seed(t, store, registry.Session{ID: "s-session", SandboxID: "s-created", Status: registry.StatusStarting,
		ProjectID: "p-original", ProjectRunID: "run-original", ProjectRunOperationID: "op-original", OpeningPrompt: "original reviewed instructions", AttachToken: "private-original"})
	before := get(t, store, "s-session")
	be := &fakeBackend{sandboxes: []backend.Sandbox{interruptedChild()}}
	r, _ := newReaper(t, store, be, 0, 0)
	if err := r.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := get(t, store, before.ID)
	if after.Status != registry.StatusExited || after.ExitReason != "sandbox creation was interrupted before the agent started; start a new attempt when ready" || after.AttachToken != "" {
		t.Fatal(after)
	}
	if after.SandboxID != before.SandboxID || after.ProjectRunID != before.ProjectRunID || after.ProjectRunOperationID != before.ProjectRunOperationID || after.OpeningPrompt != before.OpeningPrompt {
		t.Fatal("allocation provenance changed")
	}
	if be.wasDestroyed(before.SandboxID) {
		t.Fatal("terminal transition bypassed the normal cleanup sweep")
	}
	if err := r.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !be.wasDestroyed(before.SandboxID) {
		t.Fatal("terminal created child was not reclaimed")
	}
	if got := get(t, store, before.ID); got.Status != registry.StatusExited || got.ExitReason != after.ExitReason {
		t.Fatal("cleanup lost recorded interruption")
	}
}

func TestInterruptedCreateRecoveryRefusesUnconfirmedOrLiveAllocations(t *testing.T) {
	for _, test := range []string{"fresh", "future-time", "unknown-time", "active-create", "restarting", "running", "foreign", "legacy", "wrong-session", "wrong-ref", "pending-pod", "get-unavailable", "started-during-read", "changed-ref-during-read", "active-during-read", "registry-moved-during-read"} {
		t.Run(test, func(t *testing.T) {
			store := newStore(t)
			sb := interruptedChild()
			row := registry.Session{ID: "s-session", SandboxID: sb.ID, Status: registry.StatusStarting}
			switch test {
			case "fresh":
				sb.CreatedAt = clock.Add(-time.Minute)
			case "future-time":
				sb.CreatedAt = clock.Add(time.Hour)
			case "unknown-time":
				sb.CreatedAt = time.Time{}
			case "active-create":
				sb.CreationInFlight = true
			case "restarting":
				sb.CreatedOnly = false
			case "running":
				sb.State = backend.StateRunning
				sb.CreatedOnly = false
			case "foreign":
				sb.Instance = theirInstance
			case "legacy":
				sb.Instance = ""
			case "wrong-session":
				sb.Labels[backend.LabelSessionID] = "s-another"
			case "wrong-ref":
				row.SandboxRef = "different-container"
			case "pending-pod":
				sb.Kind = backend.KindKata
			}
			seed(t, store, row)
			before := get(t, store, row.ID)
			be := &creationObservationBackend{fakeBackend: &fakeBackend{sandboxes: []backend.Sandbox{sb}}}
			be.observe = func(current backend.Sandbox) (backend.Sandbox, error) {
				switch test {
				case "get-unavailable":
					return current, errors.New("daemon is restarting")
				case "started-during-read":
					current.State = backend.StateRunning
					current.CreatedOnly = false
				case "changed-ref-during-read":
					current.Ref = "replacement-container"
				case "active-during-read":
					current.CreationInFlight = true
				case "registry-moved-during-read":
					_, err := store.Update(row.ID, func(value *registry.Session) error {
						value.Status = registry.StatusRunning
						value.Endpoint = "127.0.0.1:12345"
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				return current, nil
			}
			r, _ := newReaper(t, store, be, 0, 0)
			if err := r.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			after := get(t, store, row.ID)
			if test == "registry-moved-during-read" {
				if after.Status != registry.StatusRunning || after.Endpoint != "127.0.0.1:12345" {
					t.Fatal(after)
				}
			} else if !reflect.DeepEqual(before, after) {
				t.Fatal("unconfirmed allocation was changed", test, after)
			}
			if be.wasDestroyed(sb.ID) {
				t.Fatal("live or unconfirmed allocation destroyed", test)
			}
		})
	}
}
