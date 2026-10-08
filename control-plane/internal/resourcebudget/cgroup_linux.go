//go:build linux

package resourcebudget

import (
	"errors"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type cgroupLocation struct{ directory, mount, version, controller string }

func applyCgroup(s *HostSnapshot, membershipPath, mountinfoPath string, read func(string) ([]byte, error)) {
	membership, e := read(membershipPath)
	if e != nil {
		cgroupUnavailable(s, "memory and CPU cgroup membership unavailable")
		return
	}
	mounts, e := read(mountinfoPath)
	if e != nil {
		cgroupUnavailable(s, "memory and CPU cgroup mounts unavailable")
		return
	}
	locations, e := cgroupLocations(string(membership), string(mounts))
	if e != nil {
		cgroupUnavailable(s, "cgroup location cannot be resolved safely")
		return
	}
	for _, location := range locations {
		for current, depth := location.directory, 0; ; current = filepath.Dir(current) {
			depth++
			if depth > 128 {
				cgroupUnavailable(s, "cgroup hierarchy exceeds observation limit")
				return
			}
			if location.version == "2" || location.controller == "memory" {
				applyMemoryLimit(s, current, location.version, read)
			}
			if location.version == "2" || location.controller == "cpu" {
				applyCPULimit(s, current, location.version, read)
			}
			if current == location.mount {
				break
			}
			parent := filepath.Dir(current)
			if parent == current || !within(location.mount, parent) {
				cgroupUnavailable(s, "cgroup parent is outside its mount")
				return
			}
		}
	}
}

func cgroupUnavailable(s *HostSnapshot, reason string) {
	s.Memory.EffectiveTotalBytes = nil
	s.Memory.EffectiveAvailableBytes = nil
	s.CPU.EffectiveMillis = nil
	s.Unavailable = append(s.Unavailable, Issue{"cgroup", reason})
}

func applyMemoryLimit(s *HostSnapshot, directory, version string, read func(string) ([]byte, error)) {
	maxFile, currentFile := "memory.max", "memory.current"
	if version == "1" {
		maxFile, currentFile = "memory.limit_in_bytes", "memory.usage_in_bytes"
	}
	raw, e := read(filepath.Join(directory, maxFile))
	if errors.Is(e, os.ErrNotExist) {
		return
	}
	if e != nil {
		memoryLimitUnavailable(s)
		return
	}
	text := strings.TrimSpace(string(raw))
	if text == "max" && version == "2" {
		return
	}
	limit, e := strconv.ParseUint(text, 10, 64)
	if e != nil || limit == 0 {
		memoryLimitUnavailable(s)
		return
	}
	raw, e = read(filepath.Join(directory, currentFile))
	if e != nil {
		memoryLimitUnavailable(s)
		return
	}
	usage, e := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if e != nil {
		memoryLimitUnavailable(s)
		return
	}
	available := uint64(0)
	if limit > usage {
		available = limit - usage
	}
	if s.Memory.EffectiveTotalBytes != nil && limit < *s.Memory.EffectiveTotalBytes {
		s.Memory.EffectiveTotalBytes = ptr(limit)
	}
	if s.Memory.EffectiveAvailableBytes != nil && available < *s.Memory.EffectiveAvailableBytes {
		s.Memory.EffectiveAvailableBytes = ptr(available)
	}
	s.Memory.LimitSource = "physical-and-control-plane-cgroup-v" + version
}

func memoryLimitUnavailable(s *HostSnapshot) {
	s.Memory.EffectiveTotalBytes = nil
	s.Memory.EffectiveAvailableBytes = nil
	s.Unavailable = append(s.Unavailable, Issue{"memory.effectiveAvailableBytes", "visible cgroup memory limit could not be measured"})
}

func applyCPULimit(s *HostSnapshot, directory, version string, read func(string) ([]byte, error)) {
	var quota, period string
	if version == "2" {
		raw, e := read(filepath.Join(directory, "cpu.max"))
		if errors.Is(e, os.ErrNotExist) {
			return
		}
		if e != nil {
			cpuLimitUnavailable(s)
			return
		}
		f := strings.Fields(string(raw))
		if len(f) != 2 {
			cpuLimitUnavailable(s)
			return
		}
		quota, period = f[0], f[1]
		if quota == "max" {
			return
		}
	} else {
		raw, e := read(filepath.Join(directory, "cpu.cfs_quota_us"))
		if errors.Is(e, os.ErrNotExist) {
			return
		}
		if e != nil {
			cpuLimitUnavailable(s)
			return
		}
		quota = strings.TrimSpace(string(raw))
		if quota == "-1" {
			return
		}
		raw, e = read(filepath.Join(directory, "cpu.cfs_period_us"))
		if e != nil {
			cpuLimitUnavailable(s)
			return
		}
		period = strings.TrimSpace(string(raw))
	}
	q, e1 := strconv.ParseUint(quota, 10, 64)
	p, e2 := strconv.ParseUint(period, 10, 64)
	if e1 != nil || e2 != nil || p == 0 || q == 0 || q > math.MaxInt64/1000 {
		cpuLimitUnavailable(s)
		return
	}
	millis := int64(q * 1000 / p)
	if millis <= 0 {
		cpuLimitUnavailable(s)
		return
	}
	if s.CPU.EffectiveMillis != nil && millis < *s.CPU.EffectiveMillis {
		s.CPU.EffectiveMillis = ptr(millis)
	}
	s.CPU.LimitSource = "process-affinity-and-control-plane-cgroup-v" + version
}

func cpuLimitUnavailable(s *HostSnapshot) {
	s.CPU.EffectiveMillis = nil
	s.Unavailable = append(s.Unavailable, Issue{"cpu.effectiveMillis", "visible cgroup CPU quota could not be measured"})
}

func cgroupLocations(membership, mountinfo string) ([]cgroupLocation, error) {
	groups := map[string]string{}
	for _, line := range strings.Split(membership, "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, ":", 3)
		if len(f) != 3 || !strings.HasPrefix(f[2], "/") || path.Clean(f[2]) != f[2] {
			return nil, errors.New("invalid cgroup membership")
		}
		if f[1] == "" {
			groups["2"] = f[2]
		} else {
			for _, controller := range strings.Split(f[1], ",") {
				if controller == "memory" || controller == "cpu" {
					groups[controller] = f[2]
				}
			}
		}
	}
	var result []cgroupLocation
	found := map[string]bool{}
	for _, line := range strings.Split(mountinfo, "\n") {
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) != 2 {
			continue
		}
		left, right := strings.Fields(parts[0]), strings.Fields(parts[1])
		if len(left) < 6 || len(right) < 3 || (right[0] != "cgroup2" && right[0] != "cgroup") {
			continue
		}
		root, mount := unescapeMount(left[3]), unescapeMount(left[4])
		if !filepath.IsAbs(mount) || filepath.Clean(mount) != mount {
			return nil, errors.New("invalid cgroup mount")
		}
		controllers := []string{"2"}
		version := "2"
		if right[0] == "cgroup" {
			version = "1"
			controllers = strings.Split(right[2], ",")
		}
		for _, controller := range controllers {
			group, ok := groups[controller]
			if !ok || found[controller] {
				continue
			}
			relative := ""
			if group == "/" {
				relative = "."
			} else if root == "/" {
				relative = strings.TrimPrefix(group, "/")
			} else if group == root {
				relative = "."
			} else if strings.HasPrefix(group, root+"/") {
				relative = strings.TrimPrefix(group, root+"/")
			} else {
				return nil, errors.New("cgroup namespace does not match mount")
			}
			dir := filepath.Join(mount, filepath.FromSlash(relative))
			if !within(mount, dir) {
				return nil, errors.New("cgroup directory escapes mount")
			}
			result = append(result, cgroupLocation{dir, mount, version, controller})
			found[controller] = true
		}
	}
	for controller := range groups {
		if !found[controller] {
			return nil, errors.New("cgroup controller mount unavailable")
		}
	}
	return result, nil
}

func within(root, dir string) bool {
	relative, e := filepath.Rel(root, dir)
	return e == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
func unescapeMount(s string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}
