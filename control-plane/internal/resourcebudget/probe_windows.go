//go:build windows

package resourcebudget

import (
	"errors"
	"fmt"
	"math/bits"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel                 = windows.NewLazySystemDLL("kernel32.dll")
	globalMemoryStatusEx   = kernel.NewProc("GlobalMemoryStatusEx")
	getSystemTimes         = kernel.NewProc("GetSystemTimes")
	getProcessAffinityMask = kernel.NewProc("GetProcessAffinityMask")
	getProcessMemoryInfo   = kernel.NewProc("K32GetProcessMemoryInfo")
)

type memoryStatusEx struct {
	Length, MemoryLoad                                                                                   uint32
	TotalPhys, AvailPhys, TotalPageFile, AvailPageFile, TotalVirtual, AvailVirtual, AvailExtendedVirtual uint64
}

type processMemoryCountersEx struct {
	CB, PageFaultCount                                                                                                                                                                   uint32
	PeakWorkingSetSize, WorkingSetSize, QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage, QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage, PagefileUsage, PeakPagefileUsage, PrivateUsage uintptr
}

func samplePlatform(s *HostSnapshot) cpuTicks {
	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	if result, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); result != 0 && mem.TotalPhys > 0 && mem.AvailPhys <= mem.TotalPhys {
		s.Memory = Memory{PhysicalTotalBytes: ptr(mem.TotalPhys), PhysicalAvailableBytes: ptr(mem.AvailPhys), EffectiveTotalBytes: ptr(mem.TotalPhys), EffectiveAvailableBytes: ptr(mem.AvailPhys), LimitSource: "physical"}
		// Despite the Windows field names, these are commitment values, not
		// free space in the paging file. They must not be added to physical RAM.
		if mem.TotalPageFile > 0 && mem.AvailPageFile <= mem.TotalPageFile {
			s.Memory.CommitLimitBytes = ptr(mem.TotalPageFile)
			s.Memory.CommitAvailableBytes = ptr(mem.AvailPageFile)
		}
	} else {
		s.Unavailable = append(s.Unavailable, Issue{"memory", "GlobalMemoryStatusEx failed"})
	}
	count := windows.GetActiveProcessorCount(0xffff)
	if count > 0 {
		s.CPU.LogicalProcessors = ptr(int(count))
	}
	var processMask, systemMask uintptr
	if result, _, _ := getProcessAffinityMask.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&processMask)), uintptr(unsafe.Pointer(&systemMask))); result != 0 && processMask != 0 {
		s.CPU.EffectiveMillis = ptr(int64(bits.OnesCount64(uint64(processMask))) * 1000)
		s.CPU.LimitSource = "process-affinity"
	} else {
		s.Unavailable = append(s.Unavailable, Issue{"cpu.effectiveMillis", "Windows process affinity is unavailable"})
	}
	s.Unavailable = append(s.Unavailable, Issue{"pressure", "Linux pressure stall metrics are not available on Windows"}, Issue{"memory.pageReadsPerSecond", "hard paging rate is not collected; process page faults include soft faults"})
	var idle, kernelTime, user windows.Filetime
	if result, _, _ := getSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernelTime)), uintptr(unsafe.Pointer(&user))); result != 0 {
		total := filetimeTicks(kernelTime) + filetimeTicks(user)
		idled := filetimeTicks(idle)
		if total >= idled {
			return cpuTicks{busy: total - idled, total: total, valid: true}
		}
	}
	return cpuTicks{}
}

func filetimeTicks(t windows.Filetime) uint64 {
	return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime)
}

func sampleDisk(path DiskPath) (Disk, error) {
	d := Disk{Role: path.Role}
	name, e := windows.UTF16PtrFromString(path.Path)
	if e != nil {
		return d, e
	}
	var available, total, free uint64
	if e = windows.GetDiskFreeSpaceEx(name, &available, &total, &free); e != nil {
		return d, e
	}
	if total == 0 || available > total {
		return d, errors.New("invalid volume capacity")
	}
	d.TotalBytes = ptr(total)
	d.AvailableBytes = ptr(available)
	return d, nil
}

func sampleProcess(p OwnedProcess) (ProcessUsage, error) {
	out := ProcessUsage{PID: p.PID, Role: p.Role}
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(p.PID))
	if e != nil {
		return out, e
	}
	defer windows.CloseHandle(h)
	var created, exited, kernelTime, user windows.Filetime
	if e = windows.GetProcessTimes(h, &created, &exited, &kernelTime, &user); e != nil {
		return out, e
	}
	out.StartID = fmt.Sprintf("windows-filetime:%d", filetimeTicks(created))
	if p.StartID != "" && p.StartID != out.StartID {
		return out, errors.New("process identity changed")
	}
	var counters processMemoryCountersEx
	counters.CB = uint32(unsafe.Sizeof(counters))
	if result, _, e := getProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB)); result == 0 {
		return out, e
	}
	out.ResidentBytes = ptr(uint64(counters.WorkingSetSize))
	out.PrivateCommitBytes = ptr(uint64(counters.PrivateUsage))
	return out, nil
}
