//go:build windows

package management

// Deterministic ownership-state tests only. These inject inert kernel results;
// they do not launch Ollama, create a Windows Job, prove listener ownership or
// establish actual CPU/memory enforcement or model fit.
import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

type managedModelFakeKernel struct {
	events                     []string
	id                         managedModelIdentity
	job                        managedModelJobState
	exited, terminated         bool
	holdOriginal, holdChildren bool
	code                       uint32
	limitError                 error
	createError                error
	closeErrors                map[windows.Handle]error
	closes                     []windows.Handle
	onCreate, onJobState       func()
}

func managedModelFake() *managedModelFakeKernel {
	return &managedModelFakeKernel{id: managedModelIdentity{pid: 55, created: 100, image: `C:\Tools\Ollama\ollama.exe`}, job: managedModelJobState{active: 1, pids: []uint32{55}}, closeErrors: map[windows.Handle]error{}}
}
func managedModelSpec() managedModelProcessSpec {
	return managedModelProcessSpec{executable: `C:\Tools\Ollama\ollama.exe`, directory: `E:\Luci\models`, args: []string{"serve"}, environment: map[string]string{"SystemRoot": `C:\Windows`, "OLLAMA_MODELS": `E:\Luci\models`}, generation: 7, limits: managedModelLimits{jobBytes: 3 << 30, processBytes: 2 << 30, processes: 8, cpuRate: 2000, stopTimeout: time.Second}}
}
func (f *managedModelFakeKernel) newJob() (windows.Handle, error) {
	f.events = append(f.events, "job")
	return 1, nil
}
func (f *managedModelFakeKernel) openNull() (windows.Handle, error) {
	f.events = append(f.events, "nul")
	return 2, nil
}
func (f *managedModelFakeKernel) setLimits(_ windows.Handle, _ managedModelLimits) error {
	f.events = append(f.events, "limits-readback")
	return f.limitError
}
func (f *managedModelFakeKernel) create(_ managedModelProcessSpec, job, nul windows.Handle) (managedModelCreated, error) {
	f.events = append(f.events, "create-suspended")
	if job != 1 || nul != 2 {
		return managedModelCreated{}, errors.New("wrong original startup handles")
	}
	if f.onCreate != nil {
		f.onCreate()
	}
	return managedModelCreated{process: 3, thread: 4, pid: 55}, f.createError
}
func (f *managedModelFakeKernel) identity(_ windows.Handle) (managedModelIdentity, error) {
	f.events = append(f.events, "identity")
	return f.id, nil
}
func (f *managedModelFakeKernel) jobState(_ windows.Handle) (managedModelJobState, error) {
	f.events = append(f.events, "job-state")
	if f.onJobState != nil {
		f.onJobState()
	}
	if f.terminated {
		return managedModelJobState{}, nil
	}
	return f.job, nil
}
func (f *managedModelFakeKernel) resume(_ windows.Handle) (uint32, error) {
	f.events = append(f.events, "resume")
	return 1, nil
}
func (f *managedModelFakeKernel) wait(_ windows.Handle, _ uint32) (bool, error) {
	f.events = append(f.events, "original-wait")
	return f.exited, nil
}
func (f *managedModelFakeKernel) exit(_ windows.Handle) (uint32, error) {
	f.events = append(f.events, "original-exit")
	return f.code, nil
}
func (f *managedModelFakeKernel) terminate(_ windows.Handle) error {
	f.events = append(f.events, "terminate-job")
	f.terminated = !f.holdChildren
	f.exited = !f.holdOriginal
	return nil
}
func (f *managedModelFakeKernel) close(h windows.Handle) error {
	f.closes = append(f.closes, h)
	return f.closeErrors[h]
}

func TestManagedModelLimitsRequireExactReadback(t *testing.T) {
	l := managedModelSpec().limits
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = managedModelJobFlags
	info.BasicLimitInformation.ActiveProcessLimit = l.processes
	info.JobMemoryLimit = l.jobBytes
	info.ProcessMemoryLimit = l.processBytes
	cpu := managedModelCPUInfo{flags: managedModelCPUFlags, rate: l.cpuRate}
	if !managedModelLimitsMatch(l, info, cpu) {
		t.Fatal("exact limit fixture refused")
	}
	for _, change := range []string{"breakaway", "job-memory", "process-memory", "process-count", "cpu-disabled", "cpu-weight", "cpu-rate"} {
		t.Run(change, func(t *testing.T) {
			a, b := info, cpu
			switch change {
			case "breakaway":
				a.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
			case "job-memory":
				a.JobMemoryLimit++
			case "process-memory":
				a.ProcessMemoryLimit++
			case "process-count":
				a.BasicLimitInformation.ActiveProcessLimit++
			case "cpu-disabled":
				b.flags = 0
			case "cpu-weight":
				b.flags = 1 | 2
			case "cpu-rate":
				b.rate++
			}
			if managedModelLimitsMatch(l, a, b) {
				t.Fatal("changed limits were accepted")
			}
		})
	}
}

func TestManagedModelOwnerChecksLimitsAndOriginalIdentityBeforeResume(t *testing.T) {
	f := managedModelFake()
	spec := managedModelSpec()
	o, err := startManagedModelProcessWith(context.Background(), spec, f)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"job", "limits-readback", "nul", "create-suspended", "identity", "job-state", "limits-readback", "resume"}
	if !reflect.DeepEqual(f.events, want) {
		t.Fatalf("unexpected startup ordering: %v", f.events)
	}
	spec.args[0] = "other"
	spec.environment["OLLAMA_MODELS"] = "other"
	if o.spec.args[0] != "serve" || o.spec.environment["OLLAMA_MODELS"] != `E:\Luci\models` {
		t.Fatal("caller mutation changed retained spec")
	}
	s, err := o.observe()
	if err != nil || s.generation != 7 || s.identity != f.id || s.state != "running-unverified-endpoint" || s.handlesClosed {
		t.Fatalf("unexpected running identity/status: %+v %v", s, err)
	}
	f.exited = true
	f.code = 17
	s, err = o.observe()
	if err != nil || !s.terminal || s.exitCode != 17 || s.state != "exited-unretired" {
		t.Fatalf("original exit was not observed: %+v %v", s, err)
	}
	if _, err = o.stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestManagedModelOwnerReadbackFailureNeverCreatesChild(t *testing.T) {
	f := managedModelFake()
	f.limitError = errors.New("readback differs")
	o, err := startManagedModelProcessWith(context.Background(), managedModelSpec(), f)
	if err == nil || o != nil {
		t.Fatalf("failure was not retired: %v %v", o, err)
	}
	for _, event := range f.events {
		if event == "create-suspended" || event == "resume" {
			t.Fatal("a child started after failed readback")
		}
	}
	if !reflect.DeepEqual(f.closes, []windows.Handle{1}) {
		t.Fatalf("original job not closed exactly once: %v", f.closes)
	}
}

func TestManagedModelOwnerRetainsPartialCreateHandlesOnError(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "retired", true: "close-unknown"}[unknown], func(t *testing.T) {
			f := managedModelFake()
			f.createError = errors.New("creation refused with original handles returned")
			if unknown {
				f.closeErrors[3] = errors.New("original close unknown")
			}
			o, err := startManagedModelProcessWith(context.Background(), managedModelSpec(), f)
			if err == nil || (o != nil) != unknown {
				t.Fatalf("partial originals lost or accepted: owner=%v err=%v", o, err)
			}
			for _, event := range f.events {
				if event == "resume" || event == "identity" {
					t.Fatal("failed creation continued startup")
				}
			}
			if !reflect.DeepEqual(f.closes, []windows.Handle{4, 2, 3, 1}) {
				t.Fatalf("partial original handles not all closed/attempted: %v", f.closes)
			}
		})
	}
}

func TestManagedModelOwnerCancellationNeverResumes(t *testing.T) {
	for _, when := range []string{"before-start", "during-create"} {
		t.Run(when, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := managedModelFake()
			if when == "before-start" {
				cancel()
			} else {
				f.onCreate = cancel
			}
			o, err := startManagedModelProcessWith(ctx, managedModelSpec(), f)
			if !errors.Is(err, context.Canceled) || o != nil {
				t.Fatalf("cancelled startup result: %v %v", o, err)
			}
			for _, event := range f.events {
				if event == "resume" {
					t.Fatal("cancelled child resumed")
				}
			}
			if when == "before-start" && len(f.events) != 0 {
				t.Fatal("cancelled request allocated ownership")
			}
			if when == "during-create" && (!f.terminated || len(f.closes) != 4) {
				t.Fatalf("original suspended child not retired: %v", f.events)
			}
		})
	}
}

func TestManagedModelOwnerRefusesWrongOriginalAndMembership(t *testing.T) {
	for _, change := range []string{"image", "pid", "creation", "job-pid", "job-count"} {
		t.Run(change, func(t *testing.T) {
			f := managedModelFake()
			switch change {
			case "image":
				f.id.image = `C:\Other\ollama.exe`
			case "pid":
				f.id.pid++
			case "creation":
				f.id.created = 0
			case "job-pid":
				f.job.pids = []uint32{56}
			case "job-count":
				f.job.active = 2
				f.job.pids = []uint32{55, 56}
			}
			o, err := startManagedModelProcessWith(context.Background(), managedModelSpec(), f)
			if err == nil || o != nil {
				t.Fatalf("mismatch not refused and retired: %v %v", o, err)
			}
			for _, event := range f.events {
				if event == "resume" {
					t.Fatal("unverified original resumed")
				}
			}
			if !f.terminated || len(f.closes) != 4 {
				t.Fatalf("original handles not retired: %v", f.closes)
			}
		})
	}
}

func TestManagedModelOwnerRequiresTerminalAndEmptyJobBeforeClosing(t *testing.T) {
	f := managedModelFake()
	o, err := startManagedModelProcessWith(context.Background(), managedModelSpec(), f)
	if err != nil {
		t.Fatal(err)
	}
	// An observation in progress cannot queue another unbounded owner operation.
	o.mu.Lock()
	_, busy := o.observe()
	_, stopBusy := o.stop(context.Background())
	o.mu.Unlock()
	if busy == nil || stopBusy == nil {
		t.Fatal("concurrent owner operation was not refused")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, err := o.stop(ctx)
	if !errors.Is(err, context.Canceled) || s.handlesClosed {
		t.Fatalf("cancelled observation claimed retirement: %+v %v", s, err)
	}
	deadline := o.stopDeadline
	s, err = o.stop(context.Background())
	if err != nil || !s.terminal || !s.jobEmpty || !s.handlesClosed || s.state != "stopped" {
		t.Fatalf("original retirement not joined: %+v %v", s, err)
	}
	if !deadline.Equal(o.stopDeadline) {
		t.Fatal("a second Stop reset the aggregate deadline")
	}
	if !reflect.DeepEqual(f.closes, []windows.Handle{4, 2, 3, 1}) {
		t.Fatalf("original handles not closed once: %v", f.closes)
	}
	for _, remaining := range []string{"original", "child"} {
		t.Run(remaining, func(t *testing.T) {
			f := managedModelFake()
			o, err := startManagedModelProcessWith(context.Background(), managedModelSpec(), f)
			if err != nil {
				t.Fatal(err)
			}
			if remaining == "original" {
				f.holdOriginal = true
			} else {
				f.holdChildren = true
				f.job = managedModelJobState{active: 1, pids: []uint32{56}}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.onJobState = cancel
			s, err := o.stop(ctx)
			if err == nil || s.handlesClosed || s.terminal && s.jobEmpty {
				t.Fatalf("one retirement witness was treated as both: %+v %v", s, err)
			}
			if !reflect.DeepEqual(f.closes, []windows.Handle{4, 2}) {
				t.Fatalf("unretired process/job handles were closed: %v", f.closes)
			}
		})
	}
}

func TestManagedModelOwnerCloseFailureRemainsQuarantined(t *testing.T) {
	f := managedModelFake()
	o, err := startManagedModelProcessWith(context.Background(), managedModelSpec(), f)
	if err != nil {
		t.Fatal(err)
	}
	f.closeErrors[3] = errors.New("close unknown")
	s, err := o.stop(context.Background())
	if err == nil || s.handlesClosed || s.state != "closure-unknown" || !s.terminal || !s.jobEmpty {
		t.Fatalf("failed close claimed success: %+v %v", s, err)
	}
	before := append([]windows.Handle(nil), f.closes...)
	events := len(f.events)
	if _, err = o.stop(context.Background()); err == nil {
		t.Fatal("unknown closure allowed another retirement claim")
	}
	if _, err = o.observe(); err == nil {
		t.Fatal("unknown closure was refreshed as healthy")
	}
	if !reflect.DeepEqual(before, f.closes) || len(f.events) != events {
		t.Fatal("a closed/unknown original handle was retried or queried")
	}
	if !reflect.DeepEqual(f.closes, []windows.Handle{4, 2, 3, 1}) {
		t.Fatal("one failed close skipped remaining original handles")
	}
}

func TestManagedModelSpecAndEnvironmentAreClosedAndBounded(t *testing.T) {
	for _, change := range []string{"relative-exe", "unc", "relative-directory", "generation", "job-memory", "cpu", "process-count", "stop-budget", "duplicate-env", "empty-env", "nul-arg"} {
		t.Run(change, func(t *testing.T) {
			s := managedModelSpec()
			switch change {
			case "relative-exe":
				s.executable = "ollama.exe"
			case "unc":
				s.executable = `\\host\share\ollama.exe`
			case "relative-directory":
				s.directory = "models"
			case "generation":
				s.generation = 0
			case "job-memory":
				s.limits.jobBytes = 1
			case "cpu":
				s.limits.cpuRate = 10001
			case "process-count":
				s.limits.processes = 65
			case "stop-budget":
				s.limits.stopTimeout = 31 * time.Second
			case "duplicate-env":
				s.environment["systemroot"] = "other"
			case "empty-env":
				s.environment = nil
			case "nul-arg":
				s.args = []string{"serve\x00other"}
			}
			f := managedModelFake()
			if _, err := startManagedModelProcessWith(context.Background(), s, f); err == nil || len(f.events) != 0 {
				t.Fatal("invalid input allocated native ownership")
			}
		})
	}
	block, err := managedModelEnvironment(map[string]string{"Z": "last", "A": "first"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(block, []uint16{'A', '=', 'f', 'i', 'r', 's', 't', 0, 'Z', '=', 'l', 'a', 's', 't', 0, 0}) {
		t.Fatalf("environment was inherited or not double terminated: %v", block)
	}
	if _, err = managedModelEnvironment(map[string]string{"A": strings.Repeat("x", 32767)}); err == nil {
		t.Fatal("excessive UTF-16 environment accepted")
	}
}

func TestManagedModelRevocationBeforeResumeRetiresSuspendedOriginal(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "authority", true: "cancelled-callback"}[cancelled], func(t *testing.T) {
			f := managedModelFake()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := managedModelSpec()
			s.beforeResume = func() error {
				if cancelled {
					cancel()
					return nil
				}
				return errors.New("current host permission revoked")
			}
			owner, err := startManagedModelProcessWith(ctx, s, f)
			if err == nil || owner != nil || !f.terminated {
				t.Fatal("revoked suspended child was accepted or not settled", err)
			}
			for _, event := range f.events {
				if event == "resume" {
					t.Fatal("revoked child executed")
				}
			}
		})
	}
}
