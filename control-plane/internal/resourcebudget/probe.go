package resourcebudget

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"sync"
	"time"
)

type cpuTicks struct {
	busy, total uint64
	valid       bool
}

type HostProbe struct {
	mu       sync.Mutex
	opts     ProbeOptions
	previous cpuTicks
}

func NewHostProbe(opts ProbeOptions) *HostProbe {
	opts.Disks = append([]DiskPath(nil), opts.Disks...)
	opts.OwnedProcesses = append([]OwnedProcess(nil), opts.OwnedProcesses...)
	return &HostProbe{opts: opts}
}

// ObserveDisk measures only the requested volume. The caller establishes that
// the path belongs to the scope it is reporting (host or execution daemon).
func ObserveDisk(path DiskPath) (Disk, error) {
	d, e := sampleDisk(path)
	if e == nil {
		d.ObservedAt = time.Now().UTC()
		d.Basis = "native-host-volume"
	}
	return d, e
}

func (p *HostProbe) Sample(ctx context.Context) HostSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := HostSnapshot{Platform: runtime.GOOS, Scope: "control-plane-host", Status: "unavailable", Disks: []Disk{}, Processes: []ProcessUsage{}, Unavailable: []Issue{}}
	if ctx.Err() != nil {
		s.Unavailable = append(s.Unavailable, Issue{"host", "sampling cancelled"})
		return s
	}
	ticks := samplePlatform(&s)
	if ticks.valid && p.previous.valid && ticks.total > p.previous.total && ticks.busy >= p.previous.busy {
		busy, total := ticks.busy-p.previous.busy, ticks.total-p.previous.total
		if busy <= total {
			s.CPU.UtilizationPercent = ptr(float64(busy) * 100 / float64(total))
		}
	}
	if s.CPU.UtilizationPercent == nil {
		s.Unavailable = append(s.Unavailable, Issue{"cpu.utilizationPercent", "two increasing CPU observations are required"})
	}
	p.previous = ticks
	seenRoles := map[string]bool{}
	paths := p.opts.Disks
	if p.opts.DisksFunc != nil {
		paths = p.opts.DisksFunc()
	}
	if len(paths) > 16 {
		paths = paths[:16]
		s.Unavailable = append(s.Unavailable, Issue{"disk", "configured volume observation exceeds limit"})
	}
	for _, path := range paths {
		if path.Role == "" || path.Path == "" || seenRoles[path.Role] {
			s.Unavailable = append(s.Unavailable, Issue{"disk", "each configured disk requires a unique role and local path"})
			continue
		}
		seenRoles[path.Role] = true
		disk, err := ObserveDisk(path)
		s.Disks = append(s.Disks, disk)
		if err != nil {
			s.Unavailable = append(s.Unavailable, Issue{"disk." + path.Role, "volume capacity unavailable"})
		}
	}
	processes := append([]OwnedProcess{{PID: os.Getpid(), Role: "control-plane"}}, p.opts.OwnedProcesses...)
	seenPIDs := map[int]bool{}
	for _, process := range processes {
		if seenPIDs[process.PID] {
			continue
		}
		seenPIDs[process.PID] = true
		if process.PID <= 0 || (process.PID != os.Getpid() && process.StartID == "") {
			s.Unavailable = append(s.Unavailable, Issue{"process." + process.Role, "owned process creation identity is required"})
			continue
		}
		usage, err := sampleProcess(process)
		if err != nil {
			s.Unavailable = append(s.Unavailable, Issue{"process." + process.Role, "owned process exited, changed identity, or cannot be measured"})
			continue
		}
		s.Processes = append(s.Processes, usage)
	}
	s.ObservedAt = time.Now().UTC()
	if s.Memory.EffectiveTotalBytes != nil && s.Memory.EffectiveAvailableBytes != nil && s.CPU.EffectiveMillis != nil {
		s.Status = "ready"
		if len(s.Unavailable) > 0 {
			s.Status = "partial"
		}
	}
	return s
}

// Sampler performs one shared background observation, never probes on GET, and
// never terminates a process. A slow daemon cannot accumulate pending samples.
type Sampler struct {
	probe    *HostProbe
	observer ExecutionObserver
	sampleMu sync.Mutex
	mu       sync.RWMutex
	latest   Snapshot
}

func NewSampler(probe *HostProbe, observer ExecutionObserver) *Sampler {
	return &Sampler{probe: probe, observer: observer, latest: Snapshot{Host: HostSnapshot{Status: "unavailable", Scope: "control-plane-host"}, Execution: ExecutionSnapshot{Status: "unavailable", Scope: "execution-daemon"}}}
}

func (s *Sampler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		s.SampleOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Sampler) SampleOnce(ctx context.Context) {
	s.sampleMu.Lock()
	defer s.sampleMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	next := Snapshot{Host: HostSnapshot{Status: "unavailable", Scope: "control-plane-host"}, Execution: ExecutionSnapshot{Status: "unavailable", Scope: "execution-daemon", Unavailable: []Issue{{"execution", "resource observation is unsupported or the runtime is unavailable"}}}}
	if s.probe != nil {
		next.Host = s.probe.Sample(ctx)
	}
	if s.observer != nil && ctx.Err() == nil {
		next.Execution = s.observer.ObserveResources(ctx)
	}
	s.mu.Lock()
	s.latest = next
	s.mu.Unlock()
}

// Latest returns an independent value: callers cannot mutate the shared cache
// through metric pointers or slices. The small bounded DTO contains no secrets.
func (s *Sampler) Latest() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := json.Marshal(s.latest)
	if err != nil {
		return Snapshot{Host: HostSnapshot{Status: "unavailable"}, Execution: ExecutionSnapshot{Status: "unavailable"}}
	}
	var out Snapshot
	if json.Unmarshal(data, &out) != nil {
		return Snapshot{Host: HostSnapshot{Status: "unavailable"}, Execution: ExecutionSnapshot{Status: "unavailable"}}
	}
	return out
}
