package dockerbe_test

import (
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
	"io"
	"strings"
	"testing"
	"time"
)

type resourceDocker struct {
	*fakeDocker
	statsAt      time.Time
	closedBodies int
}

func (f *resourceDocker) Info(context.Context) (system.Info, error) {
	return system.Info{ID: "engine", OSType: "linux", NCPU: 8, MemTotal: 16 << 30, Driver: "vfs"}, nil
}
func (f *resourceDocker) ContainerStatsOneShot(_ context.Context, id string) (container.StatsResponseReader, error) {
	data, _ := json.Marshal(container.StatsResponse{ID: id, Read: f.statsAt, MemoryStats: container.MemoryStats{Usage: 1000, Stats: map[string]uint64{"inactive_file": 200}}})
	return container.StatsResponseReader{Body: &resourceBody{Reader: strings.NewReader(string(data)), closed: &f.closedBodies}, OSType: "linux"}, nil
}

type resourceBody struct {
	io.Reader
	closed *int
}

func (b *resourceBody) Close() error { *b.closed++; return nil }

func TestResourceObserverCountsAllWorkloadsAndSeparatesScope(t *testing.T) {
	f := &resourceDocker{fakeDocker: &fakeDocker{list: []container.Summary{{ID: "agent", State: "running", Labels: map[string]string{backend.LabelManaged: "true", backend.LabelInstance: testInstance, backend.LabelSandboxID: "sandbox"}}, {ID: "preview", State: "running", Labels: map[string]string{"luci.preview": "true"}}, {ID: "data", State: "running", Labels: map[string]string{"ai.lcfr.luci.platform.instance": testInstance, "ai.lcfr.luci.platform.resource": "database"}}, {ID: "foreign", State: "running"}, {ID: "old", State: "exited"}}, inspects: []inspectResult{{resp: container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{HostConfig: &container.HostConfig{Resources: container.Resources{Memory: 1 << 30, NanoCPUs: 1_000_000_000}}}}}}}, statsAt: time.Now()}
	b, e := dockerbe.New(dockerbe.Config{Client: f, Instance: testInstance})
	if e != nil {
		t.Fatal(e)
	}
	got := b.ObserveResources(context.Background())
	if !got.InventoryComplete || len(got.Containers) != 4 || *got.TotalMemoryBytes != 16<<30 || got.DaemonDisk != nil {
		t.Fatalf("wrong scope/inventory: %+v", got)
	}
	if !got.Containers[0].Owned || got.Containers[1].Owned || !got.Containers[2].Owned || got.Containers[3].Owned {
		t.Fatalf("ownership invented: %+v", got.Containers)
	}
	for _, row := range got.Containers {
		if *row.MemoryUsageBytes != 1000 || *row.WorkingSetBytes != 800 || *row.MemoryLimitBytes != 1<<30 || *row.CPULimitMillis != 1000 {
			t.Fatalf("reservation confused with use: %+v", row)
		}
	}
	if f.closedBodies != 4 || len(f.listOpts) != 1 || f.listOpts[0].Filters.Len() != 0 || !f.listOpts[0].All {
		t.Fatal("did not inventory all workloads or close stats bodies")
	}
}

func TestResourceObserverUnknownLimitsAndStaleStatsRemainUnknown(t *testing.T) {
	f := &resourceDocker{fakeDocker: &fakeDocker{list: []container.Summary{{ID: "unbounded", State: "running"}}, inspects: []inspectResult{{resp: container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{HostConfig: &container.HostConfig{}}}}}}, statsAt: time.Now().Add(-time.Minute)}
	b, e := dockerbe.New(dockerbe.Config{Client: f, Instance: testInstance})
	if e != nil {
		t.Fatal(e)
	}
	got := b.ObserveResources(context.Background())
	row := got.Containers[0]
	if row.MemoryLimitBytes != nil || row.CPULimitMillis != nil || row.MemoryUsageBytes != nil || got.Status != "partial" || !got.InventoryComplete {
		t.Fatalf("unknown data invented: %+v", got)
	}
}

func TestResourceObserverRejectsTruncatedInventory(t *testing.T) {
	rows := make([]container.Summary, 129)
	for i := range rows {
		rows[i] = container.Summary{ID: "one", State: "running"}
	}
	f := &resourceDocker{fakeDocker: &fakeDocker{list: rows}, statsAt: time.Now()}
	b, e := dockerbe.New(dockerbe.Config{Client: f, Instance: testInstance})
	if e != nil {
		t.Fatal(e)
	}
	got := b.ObserveResources(context.Background())
	if got.InventoryComplete || got.Status != "unavailable" {
		t.Fatal("bounded partial inventory treated as complete")
	}
}
