//go:build windows

package management

// Pure profile/capacity/diagnostic tests: no process, Job, GPU query or model
// request is executed. Actual Windows enforcement remains a runtime gate.
import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedModelHostAdmissionKeepsIndependentPhysicalCommitAndCPU(t *testing.T) {
	p, c := uint64(9<<30), uint64(10<<30)
	cpu := 12.0
	good := resourcebudget.HostSnapshot{ObservedAt: time.Now(), Memory: resourcebudget.Memory{PhysicalAvailableBytes: &p, CommitAvailableBytes: &c}, CPU: resourcebudget.CPU{UtilizationPercent: &cpu}}
	if e := managedModelHostCapacity(good, true); e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		name   string
		change func(*resourcebudget.HostSnapshot)
	}{
		{"physical", func(h *resourcebudget.HostSnapshot) { v := uint64(7 << 30); h.Memory.PhysicalAvailableBytes = &v }},
		{"commit", func(h *resourcebudget.HostSnapshot) { v := uint64(7 << 30); h.Memory.CommitAvailableBytes = &v }},
		{"cpu", func(h *resourcebudget.HostSnapshot) { v := 50.0; h.CPU.UtilizationPercent = &v }},
		{"unknown", func(h *resourcebudget.HostSnapshot) { h.CPU.UtilizationPercent = nil }},
		{"nonfinite", func(h *resourcebudget.HostSnapshot) { v := math.NaN(); h.CPU.UtilizationPercent = &v }},
		{"stale", func(h *resourcebudget.HostSnapshot) { h.ObservedAt = time.Now().Add(-6 * time.Second) }},
		{"future", func(h *resourcebudget.HostSnapshot) { h.ObservedAt = time.Now().Add(time.Second) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := good
			test.change(&h)
			if managedModelHostCapacity(h, true) == nil {
				t.Fatal("capacity uncertainty admitted")
			}
		})
	}
	p, c = 5<<30, 5<<30
	if managedModelHostCapacity(good, false) != nil || managedModelHostCapacity(good, true) == nil {
		t.Fatal("start/stop reserve distinction lost")
	}
}
func TestManagedGPUCSVRejectsMissingChangedAndPressureObservations(t *testing.T) {
	var gpu managedModelGPU
	gpu.line([]byte("GPU-own, 8188, 7000"))
	if e := gpu.check(true); e != nil {
		t.Fatal(e)
	}
	gpu.line([]byte("GPU-own, 8188, 5000"))
	if gpu.check(true) == nil || gpu.check(false) != nil {
		t.Fatal("GPU start/stop reserves collapsed")
	}
	gpu.line([]byte("GPU-own, 8188, 500"))
	if gpu.check(false) == nil {
		t.Fatal("GPU pressure ignored")
	}
	for _, raw := range []string{"GPU-own, nope, 7000", "GPU-other, 8188, 7000", "GPU-own, 8188, 9000", "GPU-own, 8188"} {
		var g managedModelGPU
		g.line([]byte("GPU-own, 8188, 7000"))
		g.line([]byte(raw))
		if g.check(false) == nil {
			t.Fatal("malformed/changed GPU admitted")
		}
	}
	gpu = managedModelGPU{uuid: "GPU-own", free: 7 << 30, at: time.Now().Add(-6 * time.Second)}
	if gpu.check(false) == nil {
		t.Fatal("stale GPU admitted")
	}
}
func TestManagedDiagnosticsExposeOnlyFixedCodes(t *testing.T) {
	for _, c := range []struct{ raw, code string }{{"private prompt CUDA_ERROR_OUT_OF_MEMORY secret", "model_memory_unavailable"}, {"address already in use privatepath", "port_unavailable"}, {"model privatealias not found", "model_unavailable"}, {"arbitrary secret output", "startup_exit"}} {
		p := managedModelPipe{tail: []byte(c.raw)}
		code := p.failureCode()
		if code != c.code || strings.Contains(code, "secret") {
			t.Fatal("diagnostic leaked or lost its classification")
		}
		raw, _ := json.Marshal(managedmodel.Failure{Code: code})
		if strings.Contains(string(raw), c.raw) {
			t.Fatal("raw diagnostic entered public schema")
		}
	}
}
func TestManagedModelProfileAndConfigurationAreClosed(t *testing.T) {
	c := managedModelConfig{Enabled: true, ExecutablePath: `C:\Tools\Ollama\ollama.exe`, ModelStore: `E:\Models`, Port: 24567, Model: "installed:latest", Profile: managedmodel.FixedProfile()}
	if e := validateManagedModelConfig(c); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*managedModelConfig){func(c *managedModelConfig) { c.Port = 0 }, func(c *managedModelConfig) { c.Model = "bad model" }, func(c *managedModelConfig) { c.ExecutablePath = "relative.exe" }, func(c *managedModelConfig) { c.Profile.CPULimitPercent = 100 }, func(c *managedModelConfig) { c.Profile.ContextTokens = 65536 }} {
		v := c
		change(&v)
		if validateManagedModelConfig(v) == nil {
			t.Fatal("arbitrary profile/command admitted")
		}
	}
}

type stoppedManagedService struct {
	calls    int
	deadline time.Time
}

func (s *stoppedManagedService) sample(context.Context) (managedModelSample, error) {
	return managedModelSample{}, nil
}
func (s *stoppedManagedService) stop(ctx context.Context) error {
	s.calls++
	s.deadline, _ = ctx.Deadline()
	return nil
}

type inertManagedPlatform struct{}

func (inertManagedPlatform) start(context.Context, managedModelConfig, uint64) (managedModelService, error) {
	return nil, errors.New("inert test platform cannot launch")
}
func TestManagedStopChecksGenerationAndUsesOriginalCloseDeadline(t *testing.T) {
	service := &stoppedManagedService{}
	g := &Gateway{opts: Options{ManagedModelDependents: func(context.Context, string, string, bool) error { return nil }}}
	m := &managedModelManager{gateway: g, platform: inertManagedPlatform{}, path: filepath.Join(t.TempDir(), "status.json"), boot: strings.Repeat("a", 64), saved: managedModelSaved{Revision: 3}, owner: service, status: managedmodel.Status{State: "available", Generation: "7"}}
	deadline := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if m.stopExpected(ctx, false, 3, "6") == nil || service.calls != 0 || m.withdrawing {
		t.Fatal("stale stop changed current generation")
	}
	if e := m.stopExpected(ctx, true, 3, "7"); e != nil {
		t.Fatal(e)
	}
	if service.calls != 1 || !service.deadline.Equal(deadline) || m.owner != nil || m.status.State != "stopped" {
		t.Fatal("stop reset its budget or lost checked settlement")
	}
}
func TestManualManagedStopRefusesDependentWorkWithoutStoppingOwner(t *testing.T) {
	service := &stoppedManagedService{}
	g := &Gateway{opts: Options{ManagedModelDependents: func(_ context.Context, boot, generation string, stop bool) error {
		if stop || boot != "boot" || generation != "7" {
			t.Fatal("manual stop changed its scope")
		}
		return errors.New("work remains")
	}}}
	m := &managedModelManager{gateway: g, platform: inertManagedPlatform{}, path: filepath.Join(t.TempDir(), "status.json"), boot: "boot", saved: managedModelSaved{Revision: 3}, owner: service, status: managedmodel.Status{State: "available", Generation: "7"}}
	if e := m.stopExpected(context.Background(), false, 3, "7"); e == nil || service.calls != 0 || m.owner != service || m.withdrawing {
		t.Fatal("dependent work was silently killed or healthy availability was not restored")
	}
}
func TestManagedPeerRequiresExactEstablishedServerTuple(t *testing.T) {
	table := make([]byte, 28)
	binary.LittleEndian.PutUint32(table[:4], 1)
	row := table[4:]
	binary.LittleEndian.PutUint32(row[:4], 5)
	copy(row[4:8], []byte{127, 0, 0, 1})
	copy(row[12:16], []byte{127, 0, 0, 1})
	binary.BigEndian.PutUint16(row[8:10], 24567)
	binary.BigEndian.PutUint16(row[16:18], 45123)
	binary.LittleEndian.PutUint32(row[20:24], 55)
	if pid, e := managedModelPeerPID(table, 24567, 45123); e != nil || pid != 55 {
		t.Fatal("exact connected server tuple lost", pid, e)
	}
	for _, change := range []func([]byte){func(b []byte) { binary.BigEndian.PutUint16(b[20:22], 45124) }, func(b []byte) { b[8] = 0 }, func(b []byte) { binary.LittleEndian.PutUint32(b[4:8], 2) }, func(b []byte) { binary.LittleEndian.PutUint32(b[:4], 2) }} {
		copy := append([]byte(nil), table...)
		change(copy)
		if _, e := managedModelPeerPID(copy, 24567, 45123); e == nil {
			t.Fatal("different or incomplete socket ownership admitted")
		}
	}
}
