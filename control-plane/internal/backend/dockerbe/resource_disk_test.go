package dockerbe

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestDaemonDiskParsingBindsIdentityAndChecksUnits(t *testing.T) {
	good := []byte("engine-id\n/dev/vdb 10000 1000 9000 10% /var/lib/docker\n")
	d, e := parseDaemonDiskOutput(good, "engine-id")
	if e != nil || *d.TotalBytes != 10240000 || *d.AvailableBytes != 9216000 || d.ObservedAt.IsZero() {
		t.Fatalf("invalid disk units: %+v %v", d, e)
	}
	for _, raw := range []string{"other\n/dev/vdb 10000 1000 9000 10% /root", "engine-id\n/dev/vdb 10000 1000 10001 10% /root", "engine-id\n/dev/vdb 18446744073709551615 0 1 1% /root", "engine-id\n/dev/vdb no 0 0 0% /root", "engine-id\n/dev/vdb 100 1 99 1% /root\nextra"} {
		if _, e := parseDaemonDiskOutput([]byte(raw), "engine-id"); e == nil {
			t.Fatalf("invalid capacity accepted: %q", raw)
		}
	}
}

func TestDaemonDiskTargetRejectsShellAndPathAmbiguity(t *testing.T) {
	if !validDaemonDiskTarget("/var/lib/docker", "abcd-1234") {
		t.Fatal("normal daemon target rejected")
	}
	for _, root := range []string{"/", "/var/lib/../docker", "/var/lib/docker;touch", "/var/lib/docker\nother", "/var/lib/$(command)", "relative"} {
		if validDaemonDiskTarget(root, "engine-id") {
			t.Fatalf("invalid daemon target accepted: %q", root)
		}
	}
	if validDaemonDiskTarget("/var/lib/docker", "$(command)") {
		t.Fatal("invalid engine identity accepted")
	}
}

// Opt-in read-only measurement uses no Create, Start, Exec or image pull API.
func TestResourceObserverLiveDaemonReadOnly(t *testing.T) {
	if os.Getenv("LUCI_RESOURCE_PROBE_LIVE") != "1" {
		t.Skip("set LUCI_RESOURCE_PROBE_LIVE=1 to measure an existing local Docker daemon")
	}
	b, e := New(Config{Instance: "resource-probe-read-only", ResourceImage: os.Getenv("LUCI_RESOURCE_PROBE_IMAGE")})
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	snapshot := b.ObserveResources(context.Background())
	if snapshot.TotalMemoryBytes == nil || snapshot.CPUMillis == nil || !snapshot.InventoryComplete {
		t.Fatalf("daemon capacity unavailable: %+v", snapshot)
	}
	data, e := json.Marshal(snapshot)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("measured daemon baseline: %s", data)
}
