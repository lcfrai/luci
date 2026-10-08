//go:build linux

package resourcebudget

import (
	"os"
	"strings"
	"testing"
)

func TestLinuxMemoryRequiresAvailableAndRejectsOverflow(t *testing.T) {
	for _, raw := range []string{"MemTotal: 100 kB\nMemFree: 20 kB", "MemTotal: 100 kB\nMemAvailable: 101 kB", "MemTotal: 18446744073709551615 kB\nMemAvailable: 10 kB", "MemTotal: 100 MB\nMemAvailable: 10 kB"} {
		if _, _, e := parseMeminfo(raw); e == nil {
			t.Fatalf("invalid memory accepted: %q", raw)
		}
	}
	total, available, e := parseMeminfo("MemTotal: 100 kB\nMemAvailable: 20 kB")
	if e != nil || total != 102400 || available != 20480 {
		t.Fatalf("wrong memory: %d %d %v", total, available, e)
	}
}

func TestLinuxCPUDoesNotDoubleCountGuest(t *testing.T) {
	got, e := parseCPUTicks("cpu 10 20 30 40 50 60 70 80 900 1000\ncpu0 1 2 3 4")
	if e != nil || got.total != 360 || got.busy != 270 {
		t.Fatalf("wrong CPU counters: %+v %v", got, e)
	}
}

func TestLinuxPressureRequiresFiniteSome(t *testing.T) {
	for _, raw := range []string{"some avg10=NaN", "some avg10=Inf", "some avg10=101", "full avg10=1"} {
		if _, e := parsePressure(raw); e == nil {
			t.Fatalf("invalid pressure accepted: %q", raw)
		}
	}
	if v, e := parsePressure("some avg10=12.34 avg60=1 total=100"); e != nil || v != 12.34 {
		t.Fatalf("wrong pressure: %v %v", v, e)
	}
}

func TestCgroupAncestorsConstrainAvailableMemoryAndCPU(t *testing.T) {
	files := map[string]string{"membership": "0::/nested", "mounts": "1 2 0:3 / /cg rw - cgroup2 cgroup rw", "/cg/nested/memory.max": "8589934592", "/cg/nested/memory.current": "3221225472", "/cg/nested/cpu.max": "200000 100000", "/cg/memory.max": "6442450944", "/cg/memory.current": "4294967296", "/cg/cpu.max": "150000 100000"}
	s := HostSnapshot{Memory: Memory{EffectiveTotalBytes: ptr(uint64(16 << 30)), EffectiveAvailableBytes: ptr(uint64(12 << 30))}, CPU: CPU{EffectiveMillis: ptr(int64(8000))}}
	applyCgroup(&s, "membership", "mounts", fixtureReader(files))
	if *s.Memory.EffectiveTotalBytes != 6<<30 || *s.Memory.EffectiveAvailableBytes != 2<<30 || *s.CPU.EffectiveMillis != 1500 {
		t.Fatalf("ancestor limits ignored: %+v %+v", s.Memory, s.CPU)
	}
	delete(files, "/cg/memory.current")
	applyCgroup(&s, "membership", "mounts", fixtureReader(files))
	if s.Memory.EffectiveAvailableBytes != nil {
		t.Fatal("missing required usage became free RAM")
	}
}

func TestCgroupNamespaceRootAndEscapes(t *testing.T) {
	locations, e := cgroupLocations("0::/", "1 2 0:3 /.. /sys/fs/cgroup rw - cgroup2 cgroup rw")
	if e != nil || len(locations) != 1 || locations[0].directory != "/sys/fs/cgroup" {
		t.Fatalf("namespace root failed: %+v %v", locations, e)
	}
	for _, membership := range []string{"0::/../../host", "0::relative", "0::/missing"} {
		_, e := cgroupLocations(membership, "1 2 0:3 /other /cg rw - cgroup2 cgroup rw")
		if e == nil {
			t.Fatalf("unsafe cgroup accepted: %s", membership)
		}
	}
}

func TestCgroupV1QuotaAndMemory(t *testing.T) {
	files := map[string]string{"membership": "4:memory:/nested\n3:cpu,cpuacct:/nested", "mounts": "1 2 0:3 / /mem rw - cgroup cgroup rw,memory\n2 2 0:4 / /cpu rw - cgroup cgroup rw,cpu,cpuacct", "/mem/nested/memory.limit_in_bytes": "4096", "/mem/nested/memory.usage_in_bytes": "3000", "/cpu/nested/cpu.cfs_quota_us": "50000", "/cpu/nested/cpu.cfs_period_us": "100000"}
	s := HostSnapshot{Memory: Memory{EffectiveTotalBytes: ptr(uint64(8192)), EffectiveAvailableBytes: ptr(uint64(8000))}, CPU: CPU{EffectiveMillis: ptr(int64(8000))}}
	applyCgroup(&s, "membership", "mounts", fixtureReader(files))
	if *s.Memory.EffectiveAvailableBytes != 1096 || *s.CPU.EffectiveMillis != 500 {
		t.Fatalf("v1 limits ignored: %+v", s)
	}
}

func TestProcessStatHandlesParenthesesInName(t *testing.T) {
	raw := "12 (name with ) parentheses) " + strings.Repeat("0 ", 19) + "12345 0"
	got, e := parseProcessStart(raw)
	if e != nil || got != "linux-ticks:12345" {
		t.Fatalf("process identity parsed incorrectly: %s %v", got, e)
	}
}

func fixtureReader(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		v, ok := files[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(v), nil
	}
}
