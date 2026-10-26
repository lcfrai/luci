//go:build windows

package management

// This is an unexported process-ownership foundation, not a provider route.
// No caller is wired yet. The future Gateway must authorize a reviewed Ollama
// executable/configuration and fresh host capacity before calling it. Running
// here does not prove listener ownership, model readiness or GPU memory limits.
import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
)

type managedModelLimits struct {
	jobBytes, processBytes uintptr
	processes, cpuRate     uint32 // CPU rate is 1..10000 hundredths of whole-host CPU.
	stopTimeout            time.Duration
}

type managedModelProcessSpec struct {
	inheritEnvironment    bool           // Trusted fixed GPU observer only; never public configuration.
	beforeResume          func() error   // Current host authority, rechecked after all startup preparation.
	stdout, stderr        windows.Handle // Optional trusted pipe writers; caller retains and closes them.
	executable, directory string
	args                  []string
	environment           map[string]string // Exact environment; never implicitly inherited.
	generation            uint64
	limits                managedModelLimits
}

type managedModelIdentity struct {
	pid     uint32
	created uint64
	image   string
}
type managedModelCreated struct {
	process, thread windows.Handle
	pid             uint32
}
type managedModelJobState struct {
	active uint32
	pids   []uint32
}
type managedModelOwnerStatus struct {
	generation                        uint64
	identity                          managedModelIdentity
	state                             string
	observedAt                        time.Time
	exitCode                          uint32
	terminal, jobEmpty, handlesClosed bool
	// This owner deliberately captures zero log bytes, using NUL stdio.
	// A running process is never advertised as a ready model endpoint.
}

type managedModelKernel interface {
	newJob() (windows.Handle, error)
	openNull() (windows.Handle, error)
	setLimits(windows.Handle, managedModelLimits) error
	create(managedModelProcessSpec, windows.Handle, windows.Handle) (managedModelCreated, error)
	identity(windows.Handle) (managedModelIdentity, error)
	jobState(windows.Handle) (managedModelJobState, error)
	resume(windows.Handle) (uint32, error)
	wait(windows.Handle, uint32) (bool, error)
	exit(windows.Handle) (uint32, error)
	terminate(windows.Handle) error
	close(windows.Handle) error
}

type managedModelOwnedHandle struct {
	value             windows.Handle
	attempted, closed bool
}
type managedModelProcessOwner struct {
	mu                        sync.Mutex
	latest                    atomic.Value // Immutable managedModelOwnerStatus, safe during Stop.
	kernel                    managedModelKernel
	spec                      managedModelProcessSpec
	job, nul, process, thread managedModelOwnedHandle
	stopRequested             bool
	stopDeadline              time.Time
}

func validateManagedModelSpec(s managedModelProcessSpec) error {
	plainPath := func(p string) bool {
		return len(p) >= 3 && len(p) <= 32760 && p[1] == ':' && p[2] == '\\' &&
			filepath.IsAbs(p) && filepath.Clean(p) == p && !strings.ContainsAny(p, "\x00\r\n")
	}
	if !plainPath(s.executable) || !plainPath(s.directory) || s.generation == 0 || len(s.args) > 64 {
		return errors.New("managed model requires exact absolute paths and a current generation")
	}
	l := s.limits
	if l.processBytes == 0 || l.jobBytes < l.processBytes || l.processes == 0 || l.processes > 64 ||
		l.cpuRate == 0 || l.cpuRate > 10000 || l.stopTimeout <= 0 || l.stopTimeout > 30*time.Second {
		return errors.New("managed model resource bounds are invalid")
	}
	for _, arg := range s.args {
		if len(arg) > 4096 || strings.ContainsAny(arg, "\x00\r\n") {
			return errors.New("managed model argument is invalid")
		}
	}
	if !s.inheritEnvironment && len(s.environment) == 0 || s.inheritEnvironment && len(s.environment) != 0 || len(s.environment) > 64 {
		return errors.New("managed model requires an explicit bounded environment or trusted native inheritance")
	}
	seen := map[string]bool{}
	for key, value := range s.environment {
		fold := strings.ToUpper(key)
		if key == "" || seen[fold] || strings.ContainsAny(key, "=\x00\r\n") || strings.ContainsRune(value, 0) || len(key)+len(value) > 32760 {
			return errors.New("managed model environment is invalid")
		}
		seen[fold] = true
	}
	return nil
}

func startManagedModelProcess(ctx context.Context, s managedModelProcessSpec) (*managedModelProcessOwner, error) {
	return startManagedModelProcessWith(ctx, s, nativeManagedModelKernel{})
}

// A non-nil owner returned WITH an error must remain retained: retirement or
// handle closure is unknown. Never start a replacement based on that error.
func startManagedModelProcessWith(ctx context.Context, s managedModelProcessSpec, k managedModelKernel) (*managedModelProcessOwner, error) {
	if err := validateManagedModelSpec(s); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.args = append([]string(nil), s.args...)
	env := make(map[string]string, len(s.environment))
	for key, value := range s.environment {
		env[key] = value
	}
	s.environment = env
	o := &managedModelProcessOwner{kernel: k, spec: s}
	o.latest.Store(managedModelOwnerStatus{generation: s.generation, state: "starting", observedAt: time.Now()})
	fail := func(err error) (*managedModelProcessOwner, error) {
		// Cleanup has its own finite reserve even if the requesting context ended.
		cleanup, cancel := context.WithTimeout(context.Background(), s.limits.stopTimeout)
		defer cancel()
		status, stopErr := o.stop(cleanup)
		if stopErr != nil || !status.handlesClosed || !status.jobEmpty {
			return o, errors.Join(err, stopErr)
		}
		return nil, err
	}
	var err error
	o.job.value, err = k.newJob()
	if err != nil {
		return fail(err)
	}
	if o.job.value == 0 {
		return fail(errors.New("managed model job is unavailable"))
	}
	if err = k.setLimits(o.job.value, s.limits); err != nil {
		return fail(err)
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	o.nul.value, err = k.openNull()
	if err != nil {
		return fail(err)
	}
	if o.nul.value == 0 {
		return fail(errors.New("managed model stdio is unavailable"))
	}
	created, err := k.create(s, o.job.value, o.nul.value)
	// Capture returned handles before inspecting ANY fallible creation result.
	o.process.value, o.thread.value = created.process, created.thread
	status := o.latest.Load().(managedModelOwnerStatus)
	status.identity.pid = created.pid
	o.latest.Store(status)
	if err != nil {
		return fail(err)
	}
	if created.process == 0 || created.thread == 0 || created.pid == 0 {
		return fail(errors.New("managed model original handles are unavailable"))
	}
	id, err := k.identity(created.process)
	if err != nil {
		return fail(err)
	}
	if id.created == 0 || id.pid != created.pid || !strings.EqualFold(strings.TrimPrefix(id.image, `\\?\`), s.executable) {
		return fail(errors.New("managed model original identity differs"))
	}
	state, err := k.jobState(o.job.value)
	if err != nil {
		return fail(err)
	}
	if state.active != 1 || len(state.pids) != 1 || state.pids[0] != created.pid {
		return fail(errors.New("managed model atomic job membership differs"))
	}
	// Re-read exact limits after assignment and before any child code executes.
	if err = k.setLimits(o.job.value, s.limits); err != nil {
		return fail(err)
	}
	status = o.latest.Load().(managedModelOwnerStatus)
	status.identity = id
	o.latest.Store(status)
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	if s.beforeResume != nil {
		if err = s.beforeResume(); err != nil {
			return fail(err)
		}
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	previous, err := k.resume(created.thread)
	if err != nil || previous != 1 {
		return fail(errors.Join(errors.New("managed model did not resume exactly once"), err))
	}
	if err = o.closeHandle(&o.thread); err != nil {
		return fail(err)
	}
	if err = o.closeHandle(&o.nul); err != nil {
		return fail(err)
	}
	status.state, status.observedAt = "running-unverified-endpoint", time.Now()
	o.latest.Store(status)
	return o, nil
}

func (o *managedModelProcessOwner) closeHandle(h *managedModelOwnedHandle) error {
	if h.value == 0 {
		h.closed = true
		return nil
	}
	if h.attempted {
		if h.closed {
			return nil
		}
		return errors.New("managed model original handle closure is unknown")
	}
	h.attempted = true // A failed close is never retried against a possibly reused handle value.
	if err := o.kernel.close(h.value); err != nil {
		return err
	}
	h.closed = true
	return nil
}

func (o *managedModelProcessOwner) observe() (managedModelOwnerStatus, error) {
	if !o.mu.TryLock() {
		return o.latest.Load().(managedModelOwnerStatus), errors.New("managed model retirement is in progress")
	}
	defer o.mu.Unlock()
	s := o.latest.Load().(managedModelOwnerStatus)
	if s.state == "closure-unknown" {
		return s, errors.New("managed model original handle closure is unknown")
	}
	if s.handlesClosed || o.process.attempted || o.stopRequested {
		return s, nil
	}
	terminal, err := o.kernel.wait(o.process.value, 0)
	if err != nil {
		return s, err
	}
	s.terminal = terminal
	if terminal {
		s.exitCode, err = o.kernel.exit(o.process.value)
		if err != nil {
			return s, err
		}
		s.state = "exited-unretired"
	}
	s.observedAt = time.Now()
	o.latest.Store(s)
	return s, nil
}

func (o *managedModelProcessOwner) stop(ctx context.Context) (managedModelOwnerStatus, error) {
	if !o.mu.TryLock() {
		return o.latest.Load().(managedModelOwnerStatus), errors.New("managed model ownership operation is in progress")
	}
	defer o.mu.Unlock()
	s := o.latest.Load().(managedModelOwnerStatus)
	if s.handlesClosed {
		return s, nil
	}
	if s.state == "closure-unknown" {
		return s, errors.New("managed model original handle closure is unknown")
	}
	if o.stopDeadline.IsZero() {
		o.stopDeadline = time.Now().Add(o.spec.limits.stopTimeout)
	}
	deadline := o.stopDeadline
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	s.state = "stopping"
	s.observedAt = time.Now()
	o.latest.Store(s)
	if !o.stopRequested && o.job.value != 0 {
		if err := o.kernel.terminate(o.job.value); err != nil {
			return s, err
		}
		o.stopRequested = true
	}
	for {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		s.terminal = o.process.value == 0
		if !s.terminal {
			terminal, err := o.kernel.wait(o.process.value, 0)
			if err != nil {
				return s, err
			}
			s.terminal = terminal
		}
		if o.job.value == 0 {
			s.jobEmpty = true
		} else {
			state, err := o.kernel.jobState(o.job.value)
			if err != nil {
				return s, err
			}
			s.jobEmpty = state.active == 0 && len(state.pids) == 0
		}
		if s.terminal && s.jobEmpty {
			break
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return s, errors.New("managed model original retirement timed out")
		}
		if remaining > 10*time.Millisecond {
			remaining = 10 * time.Millisecond
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return s, ctx.Err()
		case <-timer.C:
		}
	}
	if o.process.value != 0 {
		code, err := o.kernel.exit(o.process.value)
		if err != nil {
			return s, err
		}
		s.exitCode = code
	}
	var closedErr error
	for _, h := range []*managedModelOwnedHandle{&o.thread, &o.nul, &o.process, &o.job} {
		closedErr = errors.Join(closedErr, o.closeHandle(h))
	}
	s.handlesClosed = o.thread.closed && o.nul.closed && o.process.closed && o.job.closed
	s.state = "stopped"
	if !s.handlesClosed {
		s.state = "closure-unknown"
	}
	s.observedAt = time.Now()
	o.latest.Store(s)
	return s, closedErr
}
