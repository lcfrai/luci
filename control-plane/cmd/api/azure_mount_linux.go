//go:build linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func azurePathDevice(path string) uint64 {
	var stat unix.Stat_t
	if unix.Stat(path, &stat) != nil {
		return 0
	}
	return uint64(stat.Dev)
}

func mountInfoUnescape(value string) string {
	for from, to := range map[string]string{`\040`: " ", `\011`: "\t", `\012`: "\n", `\134`: `\`} {
		value = strings.ReplaceAll(value, from, to)
	}
	return value
}

func azureMountInfoDevice(data []byte, mount string) (uint64, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) != 2 {
			continue
		}
		left, right := strings.Fields(parts[0]), strings.Fields(parts[1])
		if len(left) < 6 || len(right) < 2 || left[3] != "/" || mountInfoUnescape(left[4]) != mount || right[0] != "ext4" ||
			!strings.Contains(","+left[5]+",", ",rw,") || !strings.HasPrefix(right[1], "/dev/") {
			continue
		}
		numbers := strings.Split(left[2], ":")
		if len(numbers) != 2 {
			continue
		}
		major, err1 := strconv.ParseUint(numbers[0], 10, 32)
		minor, err2 := strconv.ParseUint(numbers[1], 10, 32)
		if err1 == nil && err2 == nil {
			return unix.Mkdev(uint32(major), uint32(minor)), true
		}
	}
	return 0, false
}

func azureMountedDevice(mount, uuid string) (uint64, error) {
	if !filepath.IsAbs(mount) || filepath.Clean(mount) != mount || mount == "/" || !azureUUIDPattern.MatchString(uuid) {
		return 0, errors.New("Azure mount path or filesystem UUID is invalid")
	}
	resolved, err := filepath.EvalSymlinks(mount)
	if err != nil || resolved != mount {
		return 0, errors.New("Azure disk mount is absent or redirected")
	}
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return 0, errors.New("Azure disk mount cannot be observed")
	}
	device, ok := azureMountInfoDevice(data, mount)
	if !ok || device == 0 || azurePathDevice(mount) != device {
		return 0, errors.New("Azure disk is not an exact read-write ext4 mount")
	}
	var block unix.Stat_t
	if unix.Stat("/dev/disk/by-uuid/"+uuid, &block) != nil || block.Mode&unix.S_IFMT != unix.S_IFBLK || uint64(block.Rdev) != device {
		return 0, errors.New("Azure filesystem UUID does not identify the mounted block device")
	}
	return device, nil
}
