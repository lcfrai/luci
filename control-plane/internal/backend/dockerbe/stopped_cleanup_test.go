package dockerbe_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func stoppedCleanupObserved() backend.Sandbox {
	return backend.Sandbox{ID: "sb-finished", Ref: "immutable-container", Kind: backend.KindDocker, Instance: testInstance, State: backend.StateStopped, Labels: labelsFor("sb-finished", "s-finished")}
}

func TestDestroyStoppedUsesImmutableIdentityAndNeverForce(t *testing.T) {
	observed := stoppedCleanupObserved()
	f := &fakeDocker{inspects: []inspectResult{{resp: inspectOf(observed.Ref, "exited", "127.0.0.1", "42000", observed.Labels)}}}
	if err := newBackend(t, f).DestroyStopped(context.Background(), observed); err != nil {
		t.Fatal(err)
	}
	if len(f.inspectNames) != 1 || f.inspectNames[0] != observed.Ref || len(f.removes) != 1 || f.removes[0] != observed.Ref || f.removeOpts[0].Force || !f.removeOpts[0].RemoveVolumes {
		t.Fatal("cleanup used a reusable name or force", f.inspectNames, f.removes, f.removeOpts)
	}
}

func TestDestroyStoppedRefusesChangedOwnershipOrRunningContainer(t *testing.T) {
	for _, scenario := range []string{"different ref", "running", "created", "foreign instance", "foreign session", "nonloopback", "creation pending", "missing ref"} {
		t.Run(scenario, func(t *testing.T) {
			observed := stoppedCleanupObserved()
			labels := labelsFor(observed.ID, "s-finished")
			ref, status, host := observed.Ref, "exited", "127.0.0.1"
			switch scenario {
			case "different ref":
				ref = "replacement-container"
			case "running":
				status = "running"
			case "created":
				status = "created"
			case "foreign instance":
				labels[backend.LabelInstance] = "another-instance"
			case "foreign session":
				labels[backend.LabelSessionID] = "another-session"
			case "nonloopback":
				host = "0.0.0.0"
			case "creation pending":
				observed.CreationInFlight = true
			case "missing ref":
				observed.Ref = ""
			}
			f := &fakeDocker{inspects: []inspectResult{{resp: inspectOf(ref, status, host, "42000", labels)}}}
			if err := newBackend(t, f).DestroyStopped(context.Background(), observed); err == nil {
				t.Fatal("changed or unknown container accepted")
			}
			if len(f.removes) != 0 {
				t.Fatal("uncertain container was removed", f.removes)
			}
		})
	}
}

func TestDestroyStoppedDoesNotLaunderAmbiguousAbsenceOrRemovalFailure(t *testing.T) {
	for _, phase := range []string{"inspect", "remove"} {
		for _, tc := range []struct {
			name   string
			err    error
			absent bool
		}{
			{"exact absence", notFoundErr{"old"}, true},
			{"transport", errors.New("transport unavailable"), false},
			{"joined transport", errors.Join(notFoundErr{"old"}, errors.New("transport unavailable")), false},
			{"joined canceled", errors.Join(notFoundErr{"old"}, context.Canceled), false},
			{"joined deadline", errors.Join(notFoundErr{"old"}, context.DeadlineExceeded), false},
			{"concurrent start conflict", errors.New("conflict: container is running"), false},
		} {
			t.Run(phase+"/"+tc.name, func(t *testing.T) {
				observed := stoppedCleanupObserved()
				f := &fakeDocker{inspects: []inspectResult{{resp: inspectOf(observed.Ref, "exited", "127.0.0.1", "42000", observed.Labels)}}}
				if phase == "inspect" {
					f.inspects[0].err = tc.err
				} else {
					f.removeErr = tc.err
				}
				err := newBackend(t, f).DestroyStopped(context.Background(), observed)
				if (err == nil) != tc.absent {
					t.Fatal("incorrect absence classification", err)
				}
				if phase == "inspect" && len(f.removes) != 0 {
					t.Fatal("inspection failure caused removal")
				}
				for i, id := range f.removes {
					if id != observed.Ref || f.removeOpts[i].Force {
						t.Fatal("failure retried by name/force")
					}
				}
			})
		}
	}
}
