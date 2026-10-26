//go:build windows

package management

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestManagedJobCallUsesBooleanResultAndPreservesNativeFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result uintptr
		err    error
		want   error
	}{
		{"success", 1, syscall.Errno(0), nil},
		{"success_stale_last_error", 1, windows.ERROR_ACCESS_DENIED, nil},
		{"native_failure", 0, windows.ERROR_INVALID_HANDLE, windows.ERROR_INVALID_HANDLE},
		{"failure_without_last_error", 0, syscall.Errno(0), syscall.EINVAL},
		{"failure_without_error_value", 0, nil, syscall.EINVAL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := managedModelJobCallError(tc.result, tc.err); !errors.Is(got, tc.want) {
				t.Fatalf("Job BOOL/error result = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestManagedJobInformationMatchesWindowsAMD64Layout(t *testing.T) {
	var ids managedModelPIDList
	var accounting managedModelAccounting
	var cpu managedModelCPUInfo
	if unsafe.Sizeof(uintptr(0)) != 8 || unsafe.Offsetof(ids.pids) != 8 || unsafe.Sizeof(ids) != 520 {
		t.Fatal("Job PID-list ABI differs from the supported Windows AMD64 layout")
	}
	if unsafe.Sizeof(accounting) != 48 || unsafe.Offsetof(accounting.activeProcesses) != 40 {
		t.Fatal("Job basic accounting ABI differs from Windows")
	}
	if unsafe.Sizeof(cpu) != 8 || unsafe.Offsetof(cpu.rate) != 4 {
		t.Fatal("Job CPU-rate ABI differs from Windows")
	}
}

// Explicitly admitted OS regression: creates one unnamed empty Job, sets and
// queries its limits, then closes its original handle. No process is created or
// assigned, and no model, GPU query, network request or existing Job is touched.
func TestManagedNativeEmptyJobRoundTripsTypedBuffers(t *testing.T) {
	if os.Getenv("LUCI_TEST_MANAGED_EMPTY_JOB") != "1" {
		t.Skip("requires explicit empty Windows Job scope")
	}
	kernel := nativeManagedModelKernel{}
	job, err := kernel.newJob()
	if job != 0 && job != windows.InvalidHandle {
		defer func() {
			if err := kernel.close(job); err != nil {
				t.Error("original empty Job handle closure failed:", err)
			}
		}()
	}
	if err != nil || job == 0 || job == windows.InvalidHandle {
		t.Fatal("unnamed empty Job creation failed:", err)
	}
	limits := managedModelLimits{jobBytes: 2 << 30, processBytes: 1536 << 20, processes: 4, cpuRate: 2000}
	for i := 0; i < 32; i++ {
		// Exercise real writes back into Go-owned buffers across allocations/GC.
		runtime.GC()
		if err := kernel.setLimits(job, limits); err != nil {
			t.Fatal("empty Job exact limit set/readback failed:", err)
		}
		state, err := kernel.jobState(job)
		if err != nil || state.active != 0 || len(state.pids) != 0 {
			t.Fatalf("empty Job inventory = %+v, error %v", state, err)
		}
	}
	var cpu managedModelCPUInfo
	if err := managedModelQueryJobInformation(windows.InvalidHandle, windows.JobObjectCpuRateControlInformation, unsafe.Pointer(&cpu), uint32(unsafe.Sizeof(cpu))); !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		t.Fatal("invalid original Job query did not retain native refusal:", err)
	}
	if err := managedModelSetJobInformation(windows.InvalidHandle, windows.JobObjectCpuRateControlInformation, unsafe.Pointer(&cpu), uint32(unsafe.Sizeof(cpu))); !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		t.Fatal("invalid original Job set did not retain native refusal:", err)
	}
}
