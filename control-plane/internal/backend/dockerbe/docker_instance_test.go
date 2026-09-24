package dockerbe_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/dockerbe"
)

// testInstance is the instance id every backend under test owns.
const testInstance = "11111111-1111-4111-8111-111111111111"

// otherInstance is a second control plane on the same daemon.
const otherInstance = "22222222-2222-4222-8222-222222222222"

func labelsOwnedBy(instance, sandboxID string) map[string]string {
	l := labelsFor(sandboxID, "sess-"+sandboxID)
	if instance == "" {
		delete(l, backend.LabelInstance)
	} else {
		l[backend.LabelInstance] = instance
	}
	return l
}

// task-25-26-reconcile.md item 11. The daemon holds three kinds of managed
// container: this instance's, another control plane's, and one made before
// the instance label existed. Only the first is this backend's to list —
// List is what the reaper destroys from, so a container List returns is a
// container that can be reaped.
func TestListReturnsOnlyThisInstancesContainers(t *testing.T) {
	f := &fakeDocker{list: []container.Summary{
		{ID: "c-ours", State: "running", Labels: labelsOwnedBy(testInstance, "s-ours0001")},
		{ID: "c-theirs", State: "running", Labels: labelsOwnedBy(otherInstance, "s-theirs01")},
		{ID: "c-legacy", Names: []string{"/panacea-app-s-legacy01"}, State: "running", Labels: labelsOwnedBy("", "s-legacy01")},
	}}
	b, logs := newBackendWithLog(t, f)

	got, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s-ours0001" {
		t.Fatalf("List = %+v, want only s-ours0001: anything else it returns, this control plane's reaper can destroy", got)
	}
	if got[0].Instance != testInstance {
		t.Errorf("Instance = %q, want %q: the reaper checks ownership on this field", got[0].Instance, testInstance)
	}
	if strings.Contains(logs.String(), "c-theirs") {
		t.Errorf("another control plane's container was warned about; on a shared daemon that is the normal case:\n%s", logs.String())
	}
	if n := strings.Count(logs.String(), "container=c-legacy"); n != 1 {
		t.Fatalf("the legacy container was warned about %d times, want once:\n%s", n, logs.String())
	}
	if !strings.Contains(logs.String(), "foreign") || !strings.Contains(logs.String(), backend.LabelInstance) {
		t.Errorf("the warning does not say the container is foreign and which label it lacks:\n%s", logs.String())
	}

	// A second sweep: still only ours, and the legacy container is not
	// warned about again — once per container, not two lines a minute forever.
	if got, err := b.List(context.Background()); err != nil || len(got) != 1 {
		t.Fatalf("second List = %+v, %v", got, err)
	}
	if n := strings.Count(logs.String(), "container=c-legacy"); n != 1 {
		t.Errorf("after a second List the legacy container was warned about %d times, want still once", n)
	}
}

// The warned-once set is pruned to what is on the daemon, so a long-lived
// control plane beside a busy one does not grow it forever. Docker never reuses
// a container ID, so the only way to see the pruning is to take a container
// away and put the same ID back.
func TestTheLegacyWarningSetIsPrunedToWhatIsOnTheDaemon(t *testing.T) {
	legacy := container.Summary{ID: "c-legacy", State: "running", Labels: labelsOwnedBy("", "s-legacy01")}
	f := &fakeDocker{list: []container.Summary{legacy}}
	b, logs := newBackendWithLog(t, f)

	for _, present := range []bool{true, false, true} {
		f.mu.Lock()
		f.list = nil
		if present {
			f.list = []container.Summary{legacy}
		}
		f.mu.Unlock()
		if _, err := b.List(context.Background()); err != nil {
			t.Fatalf("List: %v", err)
		}
	}
	if n := strings.Count(logs.String(), "container=c-legacy"); n != 2 {
		t.Errorf("warned %d times across present/absent/present, want 2: a container that left the daemon was never forgotten", n)
	}
}

// Get reports whatever the container carries, so a caller that asks about one
// id sees its owner rather than a blank.
func TestGetReportsTheInstanceLabel(t *testing.T) {
	f := &fakeDocker{inspects: []inspectResult{
		{resp: inspectOf("c-theirs", "running", "127.0.0.1", "51001", labelsOwnedBy(otherInstance, "s-theirs01"))},
	}}
	b := newBackend(t, f)

	sb, err := b.Get(context.Background(), "s-theirs01")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sb.Instance != otherInstance {
		t.Errorf("Instance = %q, want %q", sb.Instance, otherInstance)
	}
}

// A backend with no instance id would list every container that also has
// none — every legacy sandbox on the daemon — and hand them to the reaper. New
// refuses it, and before it asks the daemon anything: the fake's daemon is
// down, so a New that pinged first would report ErrUnavailable instead.
func TestNewRefusesAMissingOrMalformedInstance(t *testing.T) {
	for _, bad := range []string{"", "Not-Lowercase", "-leading", strings.Repeat("a", 64)} {
		f := &fakeDocker{pingErr: errors.New("cannot connect to the Docker daemon")}
		var buf bytes.Buffer
		_, err := dockerbe.New(dockerbe.Config{
			Instance:     bad,
			Client:       f,
			ReadyTimeout: time.Second,
			Log:          slog.New(slog.NewTextHandler(&buf, nil)),
		})
		if !errors.Is(err, backend.ErrUnsupported) || errors.Is(err, backend.ErrUnavailable) {
			t.Errorf("Instance %q: New = %v, want an error wrapping ErrUnsupported and not ErrUnavailable", bad, err)
		}
	}
}
