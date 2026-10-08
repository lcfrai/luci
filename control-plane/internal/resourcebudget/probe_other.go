//go:build !linux && !windows

package resourcebudget

import "errors"

func samplePlatform(s *HostSnapshot) cpuTicks {
	s.Unavailable = append(s.Unavailable, Issue{"host", "host resource observation is not implemented for this platform"})
	return cpuTicks{}
}
func sampleDisk(p DiskPath) (Disk, error) {
	return Disk{Role: p.Role}, errors.New("volume observation unsupported")
}
func sampleProcess(p OwnedProcess) (ProcessUsage, error) {
	return ProcessUsage{PID: p.PID, Role: p.Role}, errors.New("process observation unsupported")
}
