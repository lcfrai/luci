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

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
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

// task-25-26-reconcile.md item 11, and the reaper-scope fix round's I1. The
// daemon holds three kinds of managed container: this instance's, another
// control plane's, and one made before the instance label existed. List
// returns the first with Instance set and the third with Instance "", and
// skips the second.
//
// The legacy one is returned because a registry can prove it is its own when
// this backend cannot: its sandbox id was minted by one control plane at
// random, and luci-<id> is unique on the daemon. Skipping it here made
// restart recovery write `exited` over a session whose container was still
// running. Whether it may be destroyed is the reaper's call (owns()), so List
// says nothing about it: one line per sandbox, from the layer that knows.
func TestListReturnsThisInstancesAndLegacyContainersButNotAnothers(t *testing.T) {
	f := &fakeDocker{list: []container.Summary{
		{ID: "c-ours", State: "running", Labels: labelsOwnedBy(testInstance, "s-ours0001")},
		{ID: "c-theirs", State: "running", Labels: labelsOwnedBy(otherInstance, "s-theirs01")},
		{ID: "c-legacy", Names: []string{"/luci-s-legacy01"}, State: "running", Labels: labelsOwnedBy("", "s-legacy01")},
	}}
	b, logs := newBackendWithLog(t, f)

	for sweep := 1; sweep <= 2; sweep++ {
		got, err := b.List(context.Background())
		if err != nil {
			t.Fatalf("List %d: %v", sweep, err)
		}
		if len(got) != 2 || got[0].ID != "s-ours0001" || got[1].ID != "s-legacy01" {
			t.Fatalf("List %d = %+v, want s-ours0001 and s-legacy01 and never another instance's s-theirs01", sweep, got)
		}
		if got[0].Instance != testInstance {
			t.Errorf("ours: Instance = %q, want %q: the reaper checks ownership on this field", got[0].Instance, testInstance)
		}
		if got[1].Instance != "" {
			t.Errorf("legacy: Instance = %q, want empty: it carries no %s label, and the reaper must not read it as ours by label", got[1].Instance, backend.LabelInstance)
		}
		if got[1].Ref != "c-legacy" || got[1].State != backend.StateRunning {
			t.Errorf("legacy came back %+v, want Ref c-legacy and running: restart recovery matches it by sandbox id and must see it alive", got[1])
		}
	}
	if logs.Len() != 0 {
		t.Errorf("List logged about ownership; that is the reaper's line, because only it knows whether a registry here holds the id:\n%s", logs.String())
	}
}

// Reaper-scope fix round, m2. A LAN-published sandbox this control plane will
// not destroy still gets a line naming where it is published: another
// instance's at ERROR, because List skips it and nothing else here will ever
// mention it, and a legacy one returned with NonLoopback. Each once per
// container, not once per sweep.
func TestListNamesEveryOffLoopbackSandboxOnceWhoeverOwnsIt(t *testing.T) {
	lan := []container.Port{{IP: "0.0.0.0", PrivatePort: 8080, PublicPort: 51001, Type: "tcp"}}
	f := &fakeDocker{list: []container.Summary{
		{ID: "c-theirs", State: "running", Labels: labelsOwnedBy(otherInstance, "s-theirs01"), Ports: lan},
		{ID: "c-legacy", State: "running", Labels: labelsOwnedBy("", "s-legacy01"), Ports: lan},
	}}
	b, logs := newBackendWithLog(t, f)

	for sweep := 1; sweep <= 2; sweep++ {
		got, err := b.List(context.Background())
		if err != nil {
			t.Fatalf("List %d: %v", sweep, err)
		}
		if len(got) != 1 || got[0].ID != "s-legacy01" || !got[0].NonLoopback || got[0].Endpoint != "" {
			t.Fatalf("List %d = %+v, want only the legacy sandbox, flagged NonLoopback with no endpoint", sweep, got)
		}
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	var theirs, legacy []string
	for _, l := range lines {
		switch {
		case strings.Contains(l, "container=c-theirs"):
			theirs = append(theirs, l)
		case strings.Contains(l, "container=c-legacy"):
			legacy = append(legacy, l)
		}
	}
	if len(theirs) != 1 {
		t.Fatalf("another instance's exposed sandbox was logged %d times across two Lists, want once:\n%s", len(theirs), logs.String())
	}
	for _, want := range []string{"level=ERROR", "host_ip=0.0.0.0", "owner=" + otherInstance, "sandbox=s-theirs01", "loopback"} {
		if !strings.Contains(theirs[0], want) {
			t.Errorf("the line about another instance's exposed sandbox lacks %q:\n%s", want, theirs[0])
		}
	}
	if len(legacy) != 1 || !strings.Contains(legacy[0], "host_ip=0.0.0.0") {
		t.Errorf("the legacy exposed sandbox was logged %d times, want once with its host_ip:\n%s", len(legacy), logs.String())
	}
}

// The logged-once set is pruned to what is on the daemon, so a long-lived
// control plane beside a busy one does not grow it forever. Docker never reuses
// a container ID, so the only way to see the pruning is to take a container
// away and put the same ID back.
func TestTheExposureSetIsPrunedToWhatIsOnTheDaemon(t *testing.T) {
	exposed := container.Summary{ID: "c-theirs", State: "running", Labels: labelsOwnedBy(otherInstance, "s-theirs01"),
		Ports: []container.Port{{IP: "0.0.0.0", PrivatePort: 8080, PublicPort: 51001, Type: "tcp"}}}
	f := &fakeDocker{list: []container.Summary{exposed}}
	b, logs := newBackendWithLog(t, f)

	for _, present := range []bool{true, false, true} {
		f.mu.Lock()
		f.list = nil
		if present {
			f.list = []container.Summary{exposed}
		}
		f.mu.Unlock()
		if _, err := b.List(context.Background()); err != nil {
			t.Fatalf("List: %v", err)
		}
	}
	if n := strings.Count(logs.String(), "container=c-theirs"); n != 2 {
		t.Errorf("logged %d times across present/absent/present, want 2: a container that left the daemon was never forgotten", n)
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
