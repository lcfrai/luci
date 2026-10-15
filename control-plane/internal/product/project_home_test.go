package product

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func homeActor(id string) context.Context {
	return management.WithExecutionAuthority(context.Background(), management.ExecutionAuthority{Actor: management.User{ID: id, Username: id, Role: "member"}})
}

func TestHomeAcknowledgmentCapacityRemainsReadableAndExpires(t *testing.T) {
	server, err := New(Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx := homeActor("alice")
	prefs, path, err := server.homePreferenceScope(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefs.Revision = 2
	prefs.AcknowledgedStopIDs = map[string][]string{}
	prefs.AcknowledgedAt = map[string]string{}
	current := homeStopID("session", "p-next", "s-next", "generation", "2026-10-15T00:00:00Z")
	for i := 0; i < 257; i++ {
		id := fmt.Sprintf("p-%03d", i)
		prefs.AcknowledgedStopIDs[id] = []string{homeStopID("session", id, "s-old", "generation", "2026-10-15T00:00:00Z")}
		prefs.AcknowledgedAt[id] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	// A prior version could save 257 keys. Loading it must recover rather than
	// make every later Home request fail.
	if err := saveHomePreferences(path, prefs); err != nil {
		t.Fatal(err)
	}
	read, err := server.HomePreferences(ctx)
	if err != nil || len(read.AcknowledgedStopIDs) != 257 {
		t.Fatal(len(read.AcknowledgedStopIDs), err)
	}
	if _, err := server.HomeAcknowledge(ctx, "p-next", read.Revision, []string{current}, []string{current}); !errors.Is(err, ErrProjectRunLimit) {
		t.Fatal(err)
	}
	if _, err := server.HomePreferences(ctx); err != nil {
		t.Fatal("capacity refusal poisoned Home", err)
	}
	// Once an old acknowledgment ages out, capacity is recovered on the next
	// write without dropping any still-current acknowledgment.
	prefs.AcknowledgedAt["p-000"] = time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339Nano)
	prefs.AcknowledgedAt["p-001"] = prefs.AcknowledgedAt["p-000"]
	if err := saveHomePreferences(path, prefs); err != nil {
		t.Fatal(err)
	}
	ack, err := server.HomeAcknowledge(ctx, "p-next", read.Revision, []string{current}, []string{current})
	if err != nil || len(ack.AcknowledgedStopIDs) != 256 || len(ack.AcknowledgedStopIDs["p-next"]) != 1 {
		t.Fatal(len(ack.AcknowledgedStopIDs), err)
	}
}

func TestHomePreferencesAreActorScopedDurableAndCASProtected(t *testing.T) {
	state, projects := t.TempDir(), t.TempDir()
	server, err := New(Options{StateDir: state, ProjectsDir: projects})
	if err != nil {
		t.Fatal(err)
	}
	alice, bob := homeActor("alice"), homeActor("bob")
	initial, err := server.HomePreferences(alice)
	if err != nil || initial.Revision != 1 || len(initial.PinnedProjectIDs) != 0 {
		t.Fatal(initial, err)
	}
	pinned, err := server.HomePin(alice, "p-alpha", true, 1)
	if err != nil || pinned.Revision != 2 || !reflect.DeepEqual(pinned.PinnedProjectIDs, []string{"p-alpha"}) {
		t.Fatal(pinned, err)
	}
	if _, err = server.HomePin(alice, "p-beta", true, 1); !errors.Is(err, ErrProjectRunConflict) {
		t.Fatal("stale pin did not refuse", err)
	}
	other, err := server.HomePreferences(bob)
	if err != nil || other.Revision != 1 || len(other.PinnedProjectIDs) != 0 {
		t.Fatal("teammate inherited pin", other, err)
	}
	server.Close()
	reopened, err := New(Options{StateDir: state, ProjectsDir: projects})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	retained, err := reopened.HomePreferences(alice)
	if err != nil || retained.Revision != 2 || !reflect.DeepEqual(retained.PinnedProjectIDs, pinned.PinnedProjectIDs) {
		t.Fatal(retained, err)
	}
	shifted, err := New(Options{StateDir: filepath.Join(t.TempDir(), "state"), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer shifted.Close()
	isolated, err := shifted.HomePreferences(alice)
	if err != nil || len(isolated.PinnedProjectIDs) != 0 {
		t.Fatal("workspace inherited pin", isolated, err)
	}
}

func TestHomeStopWindowAcknowledgesOnlyObservedIdentity(t *testing.T) {
	server, err := New(Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	now := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	old, ok := HomeStopEvent("session", "p-alpha", "s-one", "generation-one", now.Add(-23*time.Hour).Format(time.RFC3339Nano), "stopped", now)
	if !ok {
		t.Fatal("recent stop disappeared")
	}
	if _, ok := HomeStopEvent("session", "p-alpha", "s-expired", "generation", now.Add(-24*time.Hour).Format(time.RFC3339Nano), "stopped", now); ok {
		t.Fatal("24-hour stop survived")
	}
	later, ok := HomeStopEvent("session", "p-alpha", "s-two", "generation-two", now.Format(time.RFC3339Nano), "stopped", now)
	if !ok || old.ID == later.ID {
		t.Fatal(old, later)
	}
	ack, err := server.HomeAcknowledge(homeActor("alice"), "p-alpha", 1, []string{old.ID}, []string{old.ID, later.ID})
	if err != nil || ack.Revision != 2 || !reflect.DeepEqual(ack.AcknowledgedStopIDs["p-alpha"], []string{old.ID}) {
		t.Fatal(ack, err)
	}
	if _, err = server.HomeAcknowledge(homeActor("alice"), "p-alpha", 2, []string{"stop-unknown"}, []string{later.ID}); !errors.Is(err, ErrProjectRunConflict) {
		t.Fatal("unobserved stop accepted", err)
	}
	ack, err = server.HomeAcknowledge(homeActor("alice"), "p-alpha", 2, []string{later.ID}, []string{old.ID, later.ID})
	if err != nil || !reflect.DeepEqual(ack.AcknowledgedStopIDs["p-alpha"], []string{old.ID, later.ID}) {
		t.Fatal("later stop erased earlier acknowledgment", ack, err)
	}
	reopened, err := New(Options{StateDir: server.stateDir, ProjectsDir: server.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	read, err := reopened.HomePreferences(homeActor("alice"))
	if err != nil || !reflect.DeepEqual(read.AcknowledgedStopIDs["p-alpha"], []string{old.ID, later.ID}) {
		t.Fatal("ack lost after restart", read, err)
	}
}
