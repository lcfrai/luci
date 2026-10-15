//go:build linux

package main

import (
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func readSmall(name string, max int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, errRefused
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil || int64(len(data)) > max {
		return nil, errRefused
	}
	return data, nil
}
func assemblyNullMask(name string) bool {
	switch name {
	case "/proc/interrupts", "/proc/kcore", "/proc/keys", "/proc/latency_stats", "/proc/timer_list", "/proc/timer_stats", "/proc/sched_debug":
		return true
	}
	return false
}
func validateAssemblyMounts(data []byte) error {
	required := map[string]string{"/": "ro", "/input": "ro", "/tool/luci-document-image-assembler": "ro", "/etc/hosts": "ro", "/etc/hostname": "ro", "/etc/resolv.conf": "ro", "/work": "rw", "/tmp": "rw"}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			return errRefused
		}
		separator := -1
		for i := 6; i < len(fields); i++ {
			if fields[i] == "-" {
				separator = i
				break
			}
		}
		if separator < 6 || len(fields) < separator+4 {
			return errRefused
		}
		name, options, kind := fields[4], ","+fields[5]+",", fields[separator+1]
		if desired, ok := required[name]; ok {
			if seen[name] || !strings.Contains(options, ","+desired+",") {
				return errRefused
			}
			seen[name] = true
		}
		if strings.Contains(options, ",rw,") {
			switch name {
			case "/work", "/tmp", "/dev", "/dev/shm":
				if kind != "tmpfs" {
					return errRefused
				}
			case "/proc":
				if kind != "proc" {
					return errRefused
				}
			case "/dev/pts":
				if kind != "devpts" {
					return errRefused
				}
			case "/dev/mqueue":
				if kind != "mqueue" {
					return errRefused
				}
			default:
				// Moby's fixed default kernel-file masks are bind mounts of the
				// container's /dev/null. Require both their mount identity here
				// and exact character-device1:3 stat below; never a host file.
				if !assemblyNullMask(name) || kind != "tmpfs" || fields[3] != "/null" {
					return errRefused
				}
			}
		}
	}
	for name := range required {
		if !seen[name] {
			return errRefused
		}
	}
	return nil
}
func validateAssemblyStatus(data []byte) error {
	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			if _, duplicate := fields[key]; duplicate {
				return errRefused
			}
			fields[key] = strings.TrimSpace(value)
		}
	}
	const caps = uint64((1 << 0) | (1 << 1) | (1 << 3) | (1 << 4) | (1 << 6) | (1 << 7) | (1 << 18) | (1 << 27))
	for _, key := range []string{"CapEff", "CapBnd", "CapPrm"} {
		value, err := strconv.ParseUint(fields[key], 16, 64)
		if err != nil || value != caps {
			return errRefused
		}
	}
	for _, key := range []string{"CapAmb", "CapInh"} {
		value, err := strconv.ParseUint(fields[key], 16, 64)
		if err != nil || value != 0 {
			return errRefused
		}
	}
	if fields["NoNewPrivs"] != "1" || fields["Seccomp"] != "2" {
		return errRefused
	}
	return nil
}
func verifyAssemblySandbox() error {
	mounts, err := readSmall("/proc/self/mountinfo", 256<<10)
	if err != nil || validateAssemblyMounts(mounts) != nil {
		return errRefused
	}
	for _, line := range strings.Split(string(mounts), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 5 && assemblyNullMask(fields[4]) && strings.Contains(","+fields[5]+",", ",rw,") {
			info, err := os.Lstat(fields[4])
			if err != nil || info.Mode()&os.ModeCharDevice == 0 {
				return errRefused
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Rdev != 0x103 {
				return errRefused
			}
		}
	}
	status, err := readSmall("/proc/self/status", 64<<10)
	if err != nil || validateAssemblyStatus(status) != nil {
		return errRefused
	}
	for name, want := range map[string]string{"memory.max": "2147483648", "memory.swap.max": "0", "pids.max": "64", "cpu.max": "100000 100000"} {
		data, err := readSmall("/sys/fs/cgroup/"+name, 128)
		if err != nil || strings.TrimSpace(string(data)) != want {
			return errRefused
		}
	}
	for name, want := range map[string]uint64{"/work": 2 << 30, "/tmp": 128 << 20} {
		var stat syscall.Statfs_t
		if syscall.Statfs(name, &stat) != nil || uint64(stat.Type) != 0x01021994 || stat.Bsize <= 0 || want%uint64(stat.Bsize) != 0 || stat.Blocks != want/uint64(stat.Bsize) {
			return errRefused
		}
	}
	return nil
}
