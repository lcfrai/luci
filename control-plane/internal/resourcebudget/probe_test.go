package resourcebudget

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestHostSampleMeasuresActualCapacity(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("native probe unavailable")
	}
	probe := NewHostProbe(ProbeOptions{Disks: []DiskPath{{Role: "test", Path: t.TempDir()}}, OwnedProcesses: []OwnedProcess{{PID: os.Getpid(), Role: "duplicate"}, {PID: 1, Role: "unregistered"}}})
	got := probe.Sample(context.Background())
	if got.Memory.PhysicalTotalBytes == nil || got.Memory.PhysicalAvailableBytes == nil || *got.Memory.PhysicalTotalBytes == 0 || *got.Memory.PhysicalAvailableBytes > *got.Memory.PhysicalTotalBytes {
		t.Fatalf("physical memory not observed: %+v", got)
	}
	if got.CPU.EffectiveMillis == nil || *got.CPU.EffectiveMillis <= 0 {
		t.Fatalf("effective CPU unavailable: %+v", got)
	}
	if len(got.Disks) != 1 || got.Disks[0].AvailableBytes == nil || got.Disks[0].TotalBytes == nil {
		t.Fatalf("volume not observed: %+v", got.Disks)
	}
	if len(got.Processes) != 1 || got.Processes[0].PID != os.Getpid() || got.Processes[0].StartID == "" || got.Processes[0].ResidentBytes == nil {
		t.Fatalf("owned process not observed safely: %+v", got.Processes)
	}
	data, e := json.Marshal(got)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("measured host baseline: %s", data)
}

func TestCancelledHostSampleIsNotFresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := NewHostProbe(ProbeOptions{}).Sample(ctx)
	if got.Status != "unavailable" || got.Fresh(time.Now(), time.Second) {
		t.Fatalf("cancelled sample was usable: %+v", got)
	}
}

type countObserver struct{ calls int }

func (o *countObserver) ObserveResources(context.Context) ExecutionSnapshot {
	o.calls++
	return ExecutionSnapshot{ObservedAt: time.Now(), Status: "ready", TotalMemoryBytes: ptr(uint64(1024)), Containers: []ContainerUsage{{ID: "owned", MemoryUsageBytes: ptr(uint64(256))}}, InventoryComplete: true}
}

func TestSamplerGETDoesNotProbeAndCannotMutateCache(t *testing.T) {
	observer := &countObserver{}
	sampler := NewSampler(nil, observer)
	if sampler.Latest().Host.Status != "unavailable" {
		t.Fatal("unsampled host usable")
	}
	sampler.SampleOnce(context.Background())
	first := sampler.Latest()
	*first.Execution.TotalMemoryBytes = 0
	*first.Execution.Containers[0].MemoryUsageBytes = 0
	first.Execution.Containers[0].ID = "tampered"
	for i := 0; i < 20; i++ {
		got := sampler.Latest()
		if *got.Execution.TotalMemoryBytes != 1024 || got.Execution.Containers[0].ID != "owned" || *got.Execution.Containers[0].MemoryUsageBytes != 256 {
			t.Fatal("cache can be mutated")
		}
	}
	if observer.calls != 1 {
		t.Fatalf("GET probes runtime: %d", observer.calls)
	}
}

func TestFreshnessRejectsAbsentFutureAndStale(t *testing.T) {
	now := time.Now()
	for _, at := range []time.Time{{}, now.Add(time.Second), now.Add(-6 * time.Second)} {
		if (HostSnapshot{ObservedAt: at}).Fresh(now, 5*time.Second) || (ExecutionSnapshot{ObservedAt: at}).Fresh(now, 5*time.Second) {
			t.Fatalf("invalid timestamp accepted: %s", at)
		}
	}
	if !(HostSnapshot{ObservedAt: now.Add(-time.Second)}).Fresh(now, 5*time.Second) {
		t.Fatal("fresh observation rejected")
	}
}

func TestProcessIdentityChangeIsRejected(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("native probe unavailable")
	}
	_, e := sampleProcess(OwnedProcess{PID: os.Getpid(), Role: "test", StartID: "wrong-creation-id"})
	if e == nil {
		t.Fatal("reused or foreign PID identity accepted")
	}
}

func TestDiskCallbackFollowsCurrentProjectRoot(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("native probe unavailable")
	}
	root := t.TempDir()
	probe := NewHostProbe(ProbeOptions{DisksFunc: func() []DiskPath { return []DiskPath{{Role: "projects", Path: root}} }})
	if probe.Sample(context.Background()).Disks[0].AvailableBytes == nil {
		t.Fatal("initial root unavailable")
	}
	root = ""
	next := probe.Sample(context.Background())
	if len(next.Disks) != 0 {
		t.Fatal("obsolete project root remained sampled")
	}
}
