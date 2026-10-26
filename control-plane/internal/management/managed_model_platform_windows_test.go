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

func TestManagedGPUStartupUsesFixedDiagnosticsAndOriginalExit(t *testing.T) {
	start := time.Unix(1000, 0)
	for _, test := range []struct {
		name, want        string
		free              uint64
		at                time.Time
		invalid           bool
		state             managedModelOwnerStatus
		ownerErr, pipeErr error
		wait              bool
	}{
		{name: "ready", free: 7 << 30, at: start},
		{name: "missing", want: "gpu_sample_missing", wait: true},
		{name: "low", want: "gpu_capacity_low", free: 5 << 30, at: start, wait: true},
		{name: "stale", want: "gpu_sample_stale", free: 7 << 30, at: start.Add(-6 * time.Second), wait: true},
		{name: "future", want: "gpu_sample_stale", free: 7 << 30, at: start.Add(time.Minute), wait: true},
		{name: "invalid", want: "gpu_sample_invalid", invalid: true},
		{name: "owner-unknown", want: "gpu_process_unavailable", ownerErr: errors.New("private vendor output")},
		{name: "pipe-error", want: "gpu_pipe_unavailable", pipeErr: errors.New("private vendor output")},
		{name: "exit-zero", want: "gpu_process_exited", state: managedModelOwnerStatus{terminal: true}},
		{name: "exit-17", want: "gpu_process_exited", state: managedModelOwnerStatus{terminal: true, exitCode: 17}},
	} {
		t.Run(test.name, func(t *testing.T) {
			now, pauses := start, 0
			gpu := managedModelGPU{uuid: "GPU-own", free: test.free, at: test.at, invalid: test.invalid}
			err := waitManagedGPUStartup(context.Background(), &gpu,
				func() (managedModelOwnerStatus, error) { return test.state, test.ownerErr },
				func() error { return test.pipeErr }, func() time.Time { return now },
				func(_ context.Context, delay time.Duration) error { pauses++; now = now.Add(delay); return nil })
			if test.want == "" {
				if err != nil || pauses != 0 {
					t.Fatalf("ready original refused: %v", err)
				}
				return
			}
			var detail *managedModelStartError
			if !errors.As(err, &detail) || detail.code != test.want {
				t.Fatalf("wrong fixed diagnostic: %v", err)
			}
			if test.wait && now.Sub(start) != 5*time.Second || !test.wait && pauses != 0 {
				t.Fatal("original wait budget changed or terminal/error was delayed")
			}
			if test.state.terminal {
				if detail.exitCode == nil || *detail.exitCode != test.state.exitCode {
					t.Fatal("original numeric exit lost")
				}
			} else if detail.exitCode != nil {
				t.Fatal("unknown exit invented")
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatal("raw cause exposed as diagnostic")
			}
		})
	}
}

func TestManagedGPUStartupBudgetRecoveryCancellationAndPipeEOF(t *testing.T) {
	start, now := time.Unix(2000, 0), time.Unix(2000, 0)
	var gpu managedModelGPU
	live := func() (managedModelOwnerStatus, error) { return managedModelOwnerStatus{}, nil }
	pipeOK := func() error { return nil }
	// A new insufficient observation must not start a fresh five-second budget.
	err := waitManagedGPUStartup(context.Background(), &gpu, live, pipeOK, func() time.Time { return now }, func(_ context.Context, delay time.Duration) error {
		now = now.Add(delay)
		if now.Sub(start) >= 2*time.Second {
			gpu.free, gpu.at = 5<<30, now
		}
		return nil
	})
	var detail *managedModelStartError
	if !errors.As(err, &detail) || detail.code != "gpu_capacity_low" || now.Sub(start) != 5*time.Second {
		t.Fatal("changing samples renewed startup deadline")
	}
	now = start
	gpu = managedModelGPU{}
	err = waitManagedGPUStartup(context.Background(), &gpu, live, pipeOK, func() time.Time { return now }, func(_ context.Context, delay time.Duration) error {
		now = now.Add(delay)
		if now.Sub(start) >= time.Second {
			gpu.free, gpu.at = 7<<30, now
		}
		return nil
	})
	if err != nil || now.Sub(start) != time.Second {
		t.Fatal("fresh sample could not recover within original budget")
	}
	// An observation callback that returns late cannot admit a sample outside
	// the same absolute deadline, even if that sample is individually fresh.
	now = start
	gpu = managedModelGPU{}
	err = waitManagedGPUStartup(context.Background(), &gpu, func() (managedModelOwnerStatus, error) {
		now = now.Add(6 * time.Second)
		gpu.free, gpu.at = 7<<30, now
		return managedModelOwnerStatus{}, nil
	}, pipeOK, func() time.Time { return now }, func(context.Context, time.Duration) error { t.Fatal("expired startup waited"); return nil })
	if !errors.As(err, &detail) || detail.code != "gpu_sample_missing" {
		t.Fatal("late fresh observation bypassed original budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	gpu = managedModelGPU{free: 7 << 30, at: now}
	err = waitManagedGPUStartup(ctx, &gpu, func() (managedModelOwnerStatus, error) { cancel(); return managedModelOwnerStatus{}, nil }, pipeOK, func() time.Time { return now }, func(context.Context, time.Duration) error { t.Fatal("cancelled startup waited"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled observation admitted")
	}
	done := make(chan struct{})
	close(done)
	pipe := &managedModelPipe{done: done}
	err = waitManagedGPUStartup(context.Background(), &gpu, live, pipe.observationError, func() time.Time { return now }, func(context.Context, time.Duration) error { t.Fatal("closed pipe waited"); return nil })
	if !errors.As(err, &detail) || detail.code != "gpu_pipe_unavailable" {
		t.Fatal("clean EOF mistaken for live observation")
	}
}

type failedManagedGPUPlatform struct{ failure *managedModelStartError }

func (p failedManagedGPUPlatform) start(context.Context, managedModelConfig, uint64) (managedModelService, error) {
	return nil, p.failure
}

func TestManagedGPUStartupProjectsOnlyFixedCodeAndNumericExit(t *testing.T) {
	for _, code := range []uint32{0, 17} {
		_, m := managedRouteFixture(t)
		m.platform = failedManagedGPUPlatform{failure: &managedModelStartError{code: "gpu_process_exited", exitCode: &code, cause: errors.New("private vendor output or path")}}
		done := make(chan struct{})
		m.run(context.Background(), managedModelConfig{}, 1, done)
		select {
		case <-done:
		default:
			t.Fatal("failed startup did not finish")
		}
		if m.status.State != "blocked" || m.status.Failure == nil || m.status.Failure.Code != "gpu_process_exited" || m.status.Failure.ExitCode == nil || *m.status.Failure.ExitCode != code {
			t.Fatal("actual exit not projected")
		}
		raw, err := json.Marshal(m.projectionLocked(true))
		if err != nil || strings.Contains(string(raw), "private") {
			t.Fatal("startup cause leaked")
		}
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

// The policy receives a leaf only after successful offline WinTrust, including
// exact retained-file membership for catalog signatures. This tests the closed
// route/publisher policy; the opt-in installed test exercises the real verifier.
func TestManagedModelPublisherPolicyBindsVerifiedRoute(t *testing.T) {
	const gpu = "Microsoft Windows Hardware Compatibility Publisher"
	for _, test := range []struct {
		name, vendor, route, publisher string
		allowed                       bool
	}{
		{"gpu-embedded-nvidia", gpu, "embedded", "NVIDIA Corporation", true},
		{"gpu-catalog-whcp", gpu, "catalog", gpu, true},
		{"gpu-embedded-whcp", gpu, "embedded", gpu, false},
		{"gpu-catalog-nvidia", gpu, "catalog", "NVIDIA Corporation", false},
		{"gpu-other-trusted-publisher", gpu, "embedded", "Microsoft Corporation", false},
		{"gpu-publisher-lookalike", gpu, "embedded", "NVIDIA Corporation ", false},
		{"gpu-publisher-case", gpu, "embedded", "nvidia corporation", false},
		{"gpu-unknown-route", gpu, "unknown", "NVIDIA Corporation", false},
		{"gpu-missing-route", gpu, "", "NVIDIA Corporation", false},
		{"ollama-embedded", "Ollama Inc.", "embedded", "Ollama Inc.", true},
		{"ollama-catalog-refused", "Ollama Inc.", "catalog", "Ollama Inc.", false},
		{"ollama-nvidia-refused", "Ollama Inc.", "embedded", "NVIDIA Corporation", false},
		{"unsupported-vendor-selector", "Other Vendor", "embedded", "Other Vendor", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if managedModelPublisherAllowed(test.vendor, test.route, test.publisher) != test.allowed {
				t.Fatal("verified route/publisher policy differs")
			}
		})
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
