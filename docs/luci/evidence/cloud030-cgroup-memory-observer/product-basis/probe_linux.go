//go:build linux

package resourcebudget

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func samplePlatform(s *HostSnapshot) cpuTicks {
	raw, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		total, available, e := parseMeminfo(string(raw))
		if e == nil {
			s.Memory = Memory{PhysicalTotalBytes: ptr(total), PhysicalAvailableBytes: ptr(available), EffectiveTotalBytes: ptr(total), EffectiveAvailableBytes: ptr(available), LimitSource: "physical"}
		} else {
			err = e
		}
	}
	if err != nil {
		s.Unavailable = append(s.Unavailable, Issue{"memory", "Linux MemTotal and MemAvailable are unavailable"})
	}
	var affinity unix.CPUSet
	if unix.SchedGetaffinity(0, &affinity) == nil && affinity.Count() > 0 {
		n := affinity.Count()
		s.CPU.LogicalProcessors = ptr(n)
		s.CPU.EffectiveMillis = ptr(int64(n) * 1000)
		s.CPU.LimitSource = "process-affinity"
	} else {
		s.Unavailable = append(s.Unavailable, Issue{"cpu.effectiveMillis", "process CPU affinity is unavailable"})
	}
	applyCgroup(s, "/proc/self/cgroup", "/proc/self/mountinfo", os.ReadFile)
	for _, metric := range []struct {
		name, path string
		dest       **float64
	}{{"cpu.pressureSomeAvg10", "/proc/pressure/cpu", &s.CPU.PressureSomeAvg10}, {"memory.pressureSomeAvg10", "/proc/pressure/memory", &s.CPU.MemoryPressureSomeAvg10}, {"io.pressureSomeAvg10", "/proc/pressure/io", &s.CPU.IOPressureSomeAvg10}} {
		if raw, e := os.ReadFile(metric.path); e == nil {
			if value, e := parsePressure(string(raw)); e == nil {
				*metric.dest = ptr(value)
				continue
			}
		}
		s.Unavailable = append(s.Unavailable, Issue{metric.name, "Linux pressure observation is unavailable"})
	}
	raw, err = os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTicks{}
	}
	ticks, _ := parseCPUTicks(string(raw))
	return ticks
}

func parseMeminfo(raw string) (uint64, uint64, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || (fields[0] != "MemTotal:" && fields[0] != "MemAvailable:") {
			continue
		}
		v, e := strconv.ParseUint(fields[1], 10, 64)
		if e != nil || fields[2] != "kB" || v > math.MaxUint64/1024 {
			return 0, 0, errors.New("invalid memory observation")
		}
		values[fields[0]] = v * 1024
	}
	total, t := values["MemTotal:"]
	available, a := values["MemAvailable:"]
	if !t || !a || total == 0 || available > total {
		return 0, 0, errors.New("incomplete memory observation")
	}
	return total, available, nil
}

func parseCPUTicks(raw string) (cpuTicks, error) {
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || f[0] != "cpu" {
			continue
		}
		if len(f) < 5 {
			return cpuTicks{}, errors.New("incomplete CPU counters")
		}
		var total, idle uint64
		for i := 1; i < len(f) && i <= 8; i++ {
			v, e := strconv.ParseUint(f[i], 10, 64)
			if e != nil || math.MaxUint64-total < v {
				return cpuTicks{}, errors.New("invalid CPU counters")
			}
			total += v
			if i == 4 || i == 5 {
				idle += v
			}
		}
		return cpuTicks{busy: total - idle, total: total, valid: true}, nil
	}
	return cpuTicks{}, errors.New("CPU counters unavailable")
}

func parsePressure(raw string) (float64, error) {
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || f[0] != "some" {
			continue
		}
		for _, v := range f[1:] {
			if strings.HasPrefix(v, "avg10=") {
				n, e := strconv.ParseFloat(strings.TrimPrefix(v, "avg10="), 64)
				if e == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 && n <= 100 {
					return n, nil
				}
			}
		}
	}
	return 0, errors.New("pressure unavailable")
}

func sampleDisk(path DiskPath) (Disk, error) {
	d := Disk{Role: path.Role}
	var st unix.Statfs_t
	if err := unix.Statfs(path.Path, &st); err != nil {
		return d, err
	}
	if st.Bsize <= 0 || st.Blocks > math.MaxUint64/uint64(st.Bsize) || st.Bavail > math.MaxUint64/uint64(st.Bsize) {
		return d, errors.New("invalid volume capacity")
	}
	d.TotalBytes = ptr(st.Blocks * uint64(st.Bsize))
	d.AvailableBytes = ptr(st.Bavail * uint64(st.Bsize))
	return d, nil
}

func sampleProcess(p OwnedProcess) (ProcessUsage, error) {
	out := ProcessUsage{PID: p.PID, Role: p.Role}
	base := filepath.Join("/proc", strconv.Itoa(p.PID))
	before, e := os.ReadFile(filepath.Join(base, "stat"))
	if e != nil {
		return out, e
	}
	start, e := parseProcessStart(string(before))
	if e != nil {
		return out, e
	}
	if p.StartID != "" && p.StartID != start {
		return out, errors.New("process identity changed")
	}
	statm, e := os.ReadFile(filepath.Join(base, "statm"))
	if e != nil {
		return out, e
	}
	f := strings.Fields(string(statm))
	if len(f) < 2 {
		return out, errors.New("process memory unavailable")
	}
	pages, e := strconv.ParseUint(f[1], 10, 64)
	if e != nil || pages > math.MaxUint64/uint64(os.Getpagesize()) {
		return out, errors.New("invalid process memory")
	}
	after, e := os.ReadFile(filepath.Join(base, "stat"))
	if e != nil {
		return out, e
	}
	end, e := parseProcessStart(string(after))
	if e != nil || end != start {
		return out, errors.New("process identity changed")
	}
	out.StartID = start
	out.ResidentBytes = ptr(pages * uint64(os.Getpagesize()))
	return out, nil
}

func parseProcessStart(raw string) (string, error) {
	i := strings.LastIndexByte(raw, ')')
	if i < 0 {
		return "", errors.New("invalid process stat")
	}
	f := strings.Fields(raw[i+1:])
	if len(f) <= 19 {
		return "", errors.New("incomplete process stat")
	}
	v, e := strconv.ParseUint(f[19], 10, 64)
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("linux-ticks:%d", v), nil
}
