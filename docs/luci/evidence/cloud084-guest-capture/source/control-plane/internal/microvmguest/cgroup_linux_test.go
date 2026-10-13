//go:build linux

package microvmguest

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

// This fixture drives kernel observations deterministically. It does not prove
// native freezer permission, real KVM isolation or successful root bootstrap.
type controlledTaskKernel struct {
	target        string
	boot, self    string
	number        uint64
	requested     string
	events        []string
	writes, reads int
	writeErr      error
	afterWrite    func()
	afterEvents   func()
}

func (k *controlledTaskKernel) bootID() (string, error)      { return k.boot, nil }
func (k *controlledTaskKernel) selfGroup() (string, error)   { return k.self, nil }
func (k *controlledTaskKernel) inode(string) (uint64, error) { return k.number, nil }
func (k *controlledTaskKernel) write(path, name, value string) error {
	if path != k.target || name != "cgroup.freeze" || value != "1" {
		return ErrRefused
	}
	k.writes++
	if k.writeErr != nil {
		return k.writeErr
	}
	k.requested = value
	if k.afterWrite != nil {
		k.afterWrite()
	}
	return nil
}
func (k *controlledTaskKernel) read(path, name string) (string, error) {
	if path != k.target {
		return "", ErrRefused
	}
	if name == "cgroup.freeze" {
		return k.requested, nil
	}
	if name != "cgroup.events" || len(k.events) == 0 {
		return "", ErrRefused
	}
	index := k.reads
	if index >= len(k.events) {
		index = len(k.events) - 1
	}
	k.reads++
	value := k.events[index]
	if k.afterEvents != nil {
		k.afterEvents()
	}
	return value, nil
}
func taskKernelFixture() (e2bbe.CaptureIdentity, *controlledTaskKernel) {
	id := e2bbe.CaptureIdentity{Version: 1, Identity: e2bbe.Identity{OwnerID: "owner", ProjectID: "project", TaskID: "task", AttemptID: "attempt", OperationID: "operation"}, RequestDigest: strings.Repeat("a", 64), SandboxID: "vm-fixture", LaunchDigest: strings.Repeat("b", 64), ProcessPID: 123, InputBundleDigest: strings.Repeat("c", 64), SourceDigest: strings.Repeat("d", 64), ContextDigest: strings.Repeat("e", 64), BootID: "12345678-1234-1234-1234-123456789abc", CgroupRelativePath: "luci-tasks/operation", CgroupInode: 42}
	return id, &controlledTaskKernel{target: "/sys/fs/cgroup/luci-tasks/operation", boot: id.BootID, self: "/system.slice/envd.service", number: 42, events: []string{"populated 1\nfrozen 0", "populated 1\nfrozen 1"}}
}
func TestGuestCaptureWaitsForExactKernelFreezeAndRetainsFrozenGroup(t *testing.T) {
	id, k := taskKernelFixture()
	frozen, err := freezeTaskGroup(context.Background(), id, k)
	if err != nil || frozen.Identity() != id || k.reads < 3 || k.writes != 1 || k.requested != "1" {
		t.Fatal("write intent was substituted for observed freeze", frozen, err, k)
	}
	k.events = []string{"populated 1\nfrozen 0"}
	if frozen.verifyFrozen(context.Background()) == nil {
		t.Fatal("later loss of freeze accepted")
	}
	if k.writes != 1 {
		t.Fatal("verification mutated or unfroze group")
	}
}
func TestGuestCaptureRefusesWrongKernelIdentityAndCollectorInTaskGroup(t *testing.T) {
	for _, name := range []string{"boot", "inode", "self", "descendant", "invalid-path", "write-refusal", "changed-after-write"} {
		t.Run(name, func(t *testing.T) {
			id, k := taskKernelFixture()
			effects := 0
			switch name {
			case "boot":
				k.boot = "87654321-1234-1234-1234-123456789abc"
			case "inode":
				k.number = 43
			case "self":
				k.self = "/" + id.CgroupRelativePath
			case "descendant":
				k.self = "/" + id.CgroupRelativePath + "/child"
			case "invalid-path":
				id.CgroupRelativePath = "/luci-tasks/../foreign"
			case "write-refusal":
				k.writeErr = errors.New("kernel denies freeze")
				effects = 1
			case "changed-after-write":
				k.afterWrite = func() { k.number++ }
				effects = 1
			}
			if _, err := freezeTaskGroup(context.Background(), id, k); err == nil {
				t.Fatal("unconfirmed freeze accepted")
			}
			if k.writes != effects {
				t.Fatal("unexpected freeze side effects", k.writes, effects)
			}
		})
	}
}
func TestGuestCaptureCancellationNeverGrantsCaptureOrUnfreezes(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(map[bool]string{true: "already-cancelled", false: "waiting-for-kernel"}[before], func(t *testing.T) {
			id, k := taskKernelFixture()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if before {
				cancel()
			} else {
				k.afterEvents = cancel
			}
			if _, err := freezeTaskGroup(ctx, id, k); !errors.Is(err, context.Canceled) {
				t.Fatal("cancelled capture not refused", err)
			}
			if before && k.writes != 0 || !before && (k.writes != 1 || k.requested != "1") {
				t.Fatal("cancellation released or mutated another state", k)
			}
		})
	}
}
func TestGuestCaptureRejectsAmbiguousKernelFreezeReadback(t *testing.T) {
	for _, text := range []string{"", "populated 0", "frozen 2", "frozen 1\nfrozen 0", "populated 1\nfrozen 1\nunknown 1", "frozen 1 garbage"} {
		t.Run(strings.ReplaceAll(text, "\n", "_"), func(t *testing.T) {
			id, k := taskKernelFixture()
			k.events = []string{text}
			if _, err := freezeTaskGroup(context.Background(), id, k); err == nil {
				t.Fatal("ambiguous kernel readback accepted")
			}
		})
	}
}
func TestGuestCaptureNativeRootBoundaryRefusesUnprivilegedCollector(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged refusal requires a non-root process; this does not test root freezer support")
	}
	id, _ := taskKernelFixture()
	if _, err := FreezeTaskGroup(context.Background(), id); !errors.Is(err, ErrRefused) {
		t.Fatal(err)
	}
}
