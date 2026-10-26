//go:build windows

package management

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const managedModelJobFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_JOB_MEMORY | windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY | windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
const managedModelCPUFlags uint32 = 1 | 4 // ENABLE | HARD_CAP, never weight-based scheduling.
const managedModelJobListAttribute uintptr = 0x0002000D

type managedModelCPUInfo struct{ flags, rate uint32 }
type managedModelAccounting struct {
	totalUser, totalKernel, periodUser, periodKernel                 int64
	pageFaults, totalProcesses, activeProcesses, terminatedProcesses uint32
}
type managedModelPIDList struct {
	assigned, returned uint32
	pids               [64]uintptr
}
type nativeManagedModelKernel struct{}

var managedModelKernel32 = windows.NewLazySystemDLL("kernel32.dll")

// Caller-owned aligned storage avoids an allocation escaping if the second
// InitializeProcThreadAttributeList call fails. None of these calls launches a
// process; the complete attribute list is consumed atomically by CreateProcess.
type managedModelAttributes struct {
	storage []uintptr
	list    *windows.ProcThreadAttributeList
	values  []unsafe.Pointer
}

func newManagedModelAttributes() (*managedModelAttributes, error) {
	init := managedModelKernel32.NewProc("InitializeProcThreadAttributeList")
	if err := init.Find(); err != nil {
		return nil, err
	}
	if err := managedModelKernel32.NewProc("DeleteProcThreadAttributeList").Find(); err != nil {
		return nil, err
	}
	var bytes uintptr
	r, _, err := init.Call(0, 2, 0, uintptr(unsafe.Pointer(&bytes)))
	if r != 0 || err != windows.ERROR_INSUFFICIENT_BUFFER || bytes == 0 || bytes > 64<<10 {
		return nil, errors.New("managed model startup attribute size unavailable")
	}
	a := &managedModelAttributes{storage: make([]uintptr, (bytes+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))}
	a.list = (*windows.ProcThreadAttributeList)(unsafe.Pointer(&a.storage[0]))
	r, _, err = init.Call(uintptr(unsafe.Pointer(a.list)), 2, 0, uintptr(unsafe.Pointer(&bytes)))
	if r == 0 {
		return nil, fmt.Errorf("managed model startup attributes unavailable: %w", err)
	}
	return a, nil
}
func (a *managedModelAttributes) update(key uintptr, value unsafe.Pointer, bytes uintptr) error {
	p := managedModelKernel32.NewProc("UpdateProcThreadAttribute")
	if err := p.Find(); err != nil {
		return err
	}
	// Windows retains lpValue until DeleteProcThreadAttributeList, including
	// startup failures before CreateProcess. The opaque storage is not a GC root.
	a.values = append(a.values, value)
	r, _, err := p.Call(uintptr(unsafe.Pointer(a.list)), 0, key, uintptr(value), bytes, 0, 0)
	if r == 0 {
		return fmt.Errorf("managed model startup attribute unavailable: %w", err)
	}
	return nil
}
func (a *managedModelAttributes) close() {
	managedModelKernel32.NewProc("DeleteProcThreadAttributeList").Call(uintptr(unsafe.Pointer(a.list)))
	runtime.KeepAlive(a.values)
	runtime.KeepAlive(a.storage)
}

func managedModelLimitsMatch(l managedModelLimits, info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, cpu managedModelCPUInfo) bool {
	return info.BasicLimitInformation.LimitFlags == managedModelJobFlags &&
		info.BasicLimitInformation.ActiveProcessLimit == l.processes && info.JobMemoryLimit == l.jobBytes &&
		info.ProcessMemoryLimit == l.processBytes && cpu.flags == managedModelCPUFlags && cpu.rate == l.cpuRate
}

func (nativeManagedModelKernel) newJob() (windows.Handle, error) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return h, err
	}
	if err = windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return h, err
	}
	return h, nil
}

func (nativeManagedModelKernel) openNull() (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString("NUL")
	if err != nil {
		return 0, err
	}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, &sa, windows.OPEN_EXISTING, 0, 0)
	if h == windows.InvalidHandle {
		return 0, err
	}
	return h, err
}

func (nativeManagedModelKernel) setLimits(job windows.Handle, l managedModelLimits) error {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return errors.New("managed model requires 64-bit Windows")
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = managedModelJobFlags
	info.BasicLimitInformation.ActiveProcessLimit = l.processes
	info.JobMemoryLimit, info.ProcessMemoryLimit = l.jobBytes, l.processBytes
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return err
	}
	cpu := managedModelCPUInfo{flags: managedModelCPUFlags, rate: l.cpuRate}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectCpuRateControlInformation, uintptr(unsafe.Pointer(&cpu)), uint32(unsafe.Sizeof(cpu))); err != nil {
		return err
	}
	var actual windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var actualCPU managedModelCPUInfo
	if err := windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&actual)), uint32(unsafe.Sizeof(actual)), nil); err != nil {
		return err
	}
	if err := windows.QueryInformationJobObject(job, windows.JobObjectCpuRateControlInformation, uintptr(unsafe.Pointer(&actualCPU)), uint32(unsafe.Sizeof(actualCPU)), nil); err != nil {
		return err
	}
	if !managedModelLimitsMatch(l, actual, actualCPU) {
		return errors.New("managed model job limit readback differs")
	}
	return nil
}

func managedModelEnvironment(env map[string]string) ([]uint16, error) {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.ToUpper(keys[i]) < strings.ToUpper(keys[j]) })
	var block []uint16
	for _, key := range keys {
		block = append(block, utf16.Encode([]rune(key+"="+env[key]))...)
		block = append(block, 0)
	}
	block = append(block, 0)
	if len(block) < 2 || len(block) > 32767 {
		return nil, errors.New("managed model environment exceeds finite UTF-16 bound")
	}
	return block, nil
}

func managedModelOrdinaryPath(path string, directory bool) error {
	for at := path; ; at = filepath.Dir(at) {
		p, err := windows.UTF16PtrFromString(at)
		if err != nil {
			return err
		}
		attributes, err := windows.GetFileAttributes(p)
		if err != nil {
			return err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("managed model paths cannot traverse reparse points")
		}
		if at == path && (attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory {
			return errors.New("managed model path kind differs")
		}
		if parent := filepath.Dir(at); parent == at {
			break
		}
	}
	return nil
}

func (nativeManagedModelKernel) create(s managedModelProcessSpec, job, nul windows.Handle) (managedModelCreated, error) {
	// Supply-chain authorization/pinning belongs to the future trusted caller.
	// Ordinary path checks and the retained image name do not prove binary hash,
	// model-store identity, listener ownership or resistance to same-user changes.
	// There is deliberately no spawn-then-assign or uncontained fallback.
	var result managedModelCreated
	if err := managedModelOrdinaryPath(s.executable, false); err != nil {
		return result, err
	}
	if err := managedModelOrdinaryPath(s.directory, true); err != nil {
		return result, err
	}
	argv := append([]string{s.executable}, s.args...)
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = windows.EscapeArg(arg)
	}
	command, err := windows.UTF16FromString(strings.Join(quoted, " "))
	if err != nil {
		return result, err
	}
	if len(command) > 32767 {
		return result, errors.New("managed model command line exceeds finite UTF-16 bound")
	}
	executable, err := windows.UTF16PtrFromString(s.executable)
	if err != nil {
		return result, err
	}
	directory, err := windows.UTF16PtrFromString(s.directory)
	if err != nil {
		return result, err
	}
	var environment []uint16
	var environmentPointer *uint16
	if !s.inheritEnvironment {
		environment, err = managedModelEnvironment(s.environment)
		if err != nil {
			return result, err
		}
		environmentPointer = &environment[0]
	}
	attributes, err := newManagedModelAttributes()
	if err != nil {
		return result, err
	}
	defer attributes.close()
	jobs, handles := []windows.Handle{job}, []windows.Handle{nul}
	for _, h := range []windows.Handle{s.stdout, s.stderr} {
		if h != 0 && h != nul && (len(handles) == 1 || handles[len(handles)-1] != h) {
			handles = append(handles, h)
		}
	}
	if err = attributes.update(managedModelJobListAttribute, unsafe.Pointer(&jobs[0]), unsafe.Sizeof(jobs[0])); err != nil {
		return result, err
	}
	if err = attributes.update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return result, err
	}
	si := windows.StartupInfoEx{}
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = windows.STARTF_USESTDHANDLES
	si.StdInput, si.StdOutput, si.StdErr = nul, nul, nul
	if s.stdout != 0 {
		si.StdOutput = s.stdout
	}
	if s.stderr != 0 {
		si.StdErr = s.stderr
	}
	si.ProcThreadAttributeList = attributes.list
	var pi windows.ProcessInformation
	err = windows.CreateProcess(executable, &command[0], nil, nil, true, windows.CREATE_SUSPENDED|windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_NO_WINDOW, environmentPointer, directory, &si.StartupInfo, &pi)
	runtime.KeepAlive(jobs)
	runtime.KeepAlive(handles)
	runtime.KeepAlive(attributes)
	runtime.KeepAlive(environment)
	return managedModelCreated{process: pi.Process, thread: pi.Thread, pid: pi.ProcessId}, err
}

func (nativeManagedModelKernel) identity(process windows.Handle) (managedModelIdentity, error) {
	var id managedModelIdentity
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(process, &created, &exited, &kernel, &user); err != nil {
		return id, err
	}
	pid, err := windows.GetProcessId(process)
	if err != nil {
		return id, err
	}
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err = windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil {
		return id, err
	}
	if size == 0 || size >= uint32(len(buffer)) {
		return id, errors.New("managed model original image identity unavailable")
	}
	return managedModelIdentity{pid: pid, created: uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), image: windows.UTF16ToString(buffer[:size])}, nil
}

func (nativeManagedModelKernel) jobState(job windows.Handle) (managedModelJobState, error) {
	var ids managedModelPIDList
	var accounting managedModelAccounting
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&ids)), uint32(unsafe.Sizeof(ids)), nil); err != nil {
		return managedModelJobState{}, err
	}
	if ids.assigned != ids.returned || ids.returned > uint32(len(ids.pids)) {
		return managedModelJobState{}, errors.New("managed model complete job inventory unavailable")
	}
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
		return managedModelJobState{}, err
	}
	if accounting.activeProcesses > 64 {
		return managedModelJobState{}, errors.New("managed model active process census exceeds bound")
	}
	state := managedModelJobState{active: accounting.activeProcesses}
	for _, pid := range ids.pids[:ids.returned] {
		if pid == 0 || pid > 0xffffffff {
			return managedModelJobState{}, errors.New("managed model process identity exceeds bound")
		}
		state.pids = append(state.pids, uint32(pid))
	}
	return state, nil
}

func (nativeManagedModelKernel) resume(h windows.Handle) (uint32, error) {
	return windows.ResumeThread(h)
}
func (nativeManagedModelKernel) wait(h windows.Handle, ms uint32) (bool, error) {
	state, err := windows.WaitForSingleObject(h, ms)
	if err != nil {
		return false, err
	}
	if state == windows.WAIT_OBJECT_0 {
		return true, nil
	}
	if state == uint32(windows.WAIT_TIMEOUT) {
		return false, nil
	}
	return false, fmt.Errorf("managed model original wait returned %d", state)
}
func (nativeManagedModelKernel) exit(h windows.Handle) (uint32, error) {
	var code uint32
	err := windows.GetExitCodeProcess(h, &code)
	return code, err
}
func (nativeManagedModelKernel) terminate(h windows.Handle) error {
	return windows.TerminateJobObject(h, 125)
}
func (nativeManagedModelKernel) close(h windows.Handle) error { return windows.CloseHandle(h) }
