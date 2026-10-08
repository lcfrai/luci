package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type capacityWorkerEvent struct {
	Event         string `json:"event"`
	Project       string `json:"project"`
	Task          string `json:"task"`
	PID           int    `json:"pid"`
	StartID       string `json:"startId,omitempty"`
	ContextDigest string `json:"contextDigest"`
	Iterations    int    `json:"iterations"`
	Payload       string `json:"payload,omitempty"`
}

// A controlled local process, never a model. The same binary runs on Windows.
// It reads its real private capture, touches 16 MiB, mixes CPU/file/output work,
// and only produces a final result after an explicit parent command.
func TestSwarmCapacityWorkerProcess(t *testing.T) {
	if os.Getenv("LUCI_SWARM_CAPACITY_WORKER") != "1" {
		return
	}
	projectID, task, root := os.Getenv("LUCI_SWARM_CAPACITY_PROJECT"), os.Getenv("LUCI_SWARM_CAPACITY_TASK"), os.Getenv("LUCI_SWARM_CAPACITY_CONTEXT")
	data, err := os.ReadFile(filepath.Join(root, ".luci", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan product.ProjectPlan
	if json.Unmarshal(data, &plan) != nil || plan.ProjectID != projectID {
		t.Fatal("foreign plan context")
	}
	brief, err := os.ReadFile(filepath.Join(root, "LUCI.md"))
	if err != nil {
		t.Fatal(err)
	}
	marker := bytes.Index(brief, []byte("## Assigned task contract"))
	if marker < 0 {
		t.Fatal("missing task contract")
	}
	markerJSON := bytes.Index(brief[marker:], []byte("\n```json\n"))
	if markerJSON < 0 {
		t.Fatal("missing task contract JSON")
	}
	var contract struct {
		SelectedTasks []struct {
			ID         string   `json:"id"`
			WritePaths []string `json:"writePaths"`
		} `json:"selectedTasks"`
	}
	if json.NewDecoder(bytes.NewReader(brief[marker+markerJSON+len("\n```json\n"):])).Decode(&contract) != nil || len(contract.SelectedTasks) != 1 || contract.SelectedTasks[0].ID != task || len(contract.SelectedTasks[0].WritePaths) != 1 || contract.SelectedTasks[0].WritePaths[0] != task+"-result.txt" {
		t.Fatal("foreign task assignment")
	}
	digest := sha256.Sum256(brief)
	event := capacityWorkerEvent{Project: projectID, Task: task, PID: os.Getpid(), ContextDigest: hex.EncodeToString(digest[:])}
	sample := resourcebudget.NewHostProbe(resourcebudget.ProbeOptions{}).Sample(context.Background())
	if len(sample.Processes) > 0 {
		event.StartID = sample.Processes[0].StartID
	}
	emit := func(kind, payload string) {
		event.Event, event.Payload = kind, payload
		if json.NewEncoder(os.Stdout).Encode(event) != nil {
			os.Exit(3)
		}
	}
	memory := make([]byte, 16<<20)
	for i := 0; i < len(memory); i += 4096 {
		memory[i] = byte(i / 4096)
	}
	commands := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			commands <- scanner.Text()
		}
		close(commands)
	}()
	emit("ready", "")
	if command := <-commands; command != "start" {
		t.Fatal("missing workload release")
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case command, open := <-commands:
			if !open {
				os.Exit(4)
			}
			if command == "fail" {
				emit("failed", "")
				os.Exit(2)
			}
			if command != "finish" {
				t.Fatal("unexpected worker command")
			}
			result := capacityWorkerEvent{Event: "result", Project: projectID, Task: task, ContextDigest: event.ContextDigest}
			output, _ := json.Marshal(result)
			if os.WriteFile(task+"-result.txt", append(output, '\n'), 0600) != nil {
				os.Exit(5)
			}
			emit("completed", "")
			runtime.KeepAlive(memory)
			os.Exit(0)
		case <-ticker.C:
			for range 5000 {
				digest = sha256.Sum256(digest[:])
			}
			event.Iterations++
			memory[(event.Iterations*4096)%len(memory)]++
			if task == "ui" || event.Iterations%5 == 0 {
				if os.WriteFile(task+"-result.txt", memory[:64<<10], 0600) != nil {
					os.Exit(6)
				}
			}
			if event.Iterations%5 == 0 {
				emit("progress", strings.Repeat("controlled-output ", 64))
			}
		}
	}
}

type capacityWorker struct {
	spec              backend.Spec
	session           registry.Session
	command           *exec.Cmd
	stdin             io.WriteCloser
	ready             chan capacityWorkerEvent
	done              chan struct{}
	mu                sync.Mutex
	events            int
	outputBytes       int64
	event             capacityWorkerEvent
	exitErr           error
	diagnosticOutput  string
	createError       string
	destroyRequested  bool
	started           time.Time
	exited            time.Time
	path, contextRoot string
	stderr            bytes.Buffer
}
type capacityProcessBackend struct {
	mu      sync.Mutex
	store   func() registry.Store
	workers map[string]*capacityWorker
	order   []string
	peak    int
}

func (b *capacityProcessBackend) Kind() backend.Kind { return backend.KindDocker }
func (b *capacityProcessBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	row, err := b.store().Get(spec.SessionID)
	if err != nil || row.ImplementationContext == nil || len(row.ImplementationContext.TaskIDs) != 1 {
		return backend.Sandbox{}, errors.New("worker has no durable singleton assignment")
	}
	w := &capacityWorker{spec: spec, session: row, ready: make(chan capacityWorkerEvent, 1), done: make(chan struct{}), started: time.Now()}
	for _, mount := range spec.Mounts {
		if mount.ContainerPath == sessionWorkDir {
			w.path = mount.HostPath
		}
		if mount.ContainerPath == reviewedContextContainerDir && mount.ReadOnly {
			w.contextRoot = mount.HostPath
		}
	}
	if w.path == "" || w.contextRoot == "" || w.path == w.contextRoot {
		return backend.Sandbox{}, errors.New("missing isolated source or captured context")
	}
	executable, err := os.Executable()
	if err != nil {
		return backend.Sandbox{}, err
	}
	w.command = exec.Command(executable, "-test.run=^TestSwarmCapacityWorkerProcess$")
	w.command.Dir = w.path
	w.command.Env = append(os.Environ(), "LUCI_SWARM_CAPACITY_WORKER=1", "LUCI_SWARM_CAPACITY_PROJECT="+row.ProjectID, "LUCI_SWARM_CAPACITY_TASK="+row.ImplementationContext.TaskIDs[0], "LUCI_SWARM_CAPACITY_CONTEXT="+w.contextRoot)
	w.command.Stderr = &w.stderr
	w.stdin, err = w.command.StdinPipe()
	if err != nil {
		return backend.Sandbox{}, err
	}
	output, err := w.command.StdoutPipe()
	if err != nil {
		return backend.Sandbox{}, err
	}
	if err = w.command.Start(); err != nil {
		return backend.Sandbox{}, err
	}
	b.mu.Lock()
	if b.workers == nil {
		b.workers = map[string]*capacityWorker{}
	}
	b.workers[spec.ID] = w
	b.order = append(b.order, spec.ID)
	live := 0
	for _, other := range b.workers {
		select {
		case <-other.done:
		default:
			live++
		}
	}
	b.peak = max(b.peak, live)
	b.mu.Unlock()
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			raw := scanner.Bytes()
			var event capacityWorkerEvent
			if json.Unmarshal(raw, &event) != nil {
				w.mu.Lock()
				if len(w.diagnosticOutput) < 4096 {
					w.diagnosticOutput += string(raw[:min(len(raw), 4096-len(w.diagnosticOutput))])
				}
				w.mu.Unlock()
				continue
			}
			w.mu.Lock()
			w.event = event
			w.events++
			w.outputBytes += int64(len(raw) + 1)
			w.mu.Unlock()
			if event.Event == "ready" {
				w.ready <- event
			}
		}
		err := w.command.Wait()
		w.mu.Lock()
		w.exitErr = err
		w.exited = time.Now()
		w.mu.Unlock()
		close(w.done)
	}()
	select {
	case <-w.ready:
		return b.Get(ctx, spec.ID)
	case <-w.done:
		return backend.Sandbox{}, fmt.Errorf("controlled worker exited before readiness: %s", w.stderr.String())
	case <-ctx.Done():
		w.mu.Lock()
		w.createError = ctx.Err().Error()
		w.mu.Unlock()
		_ = w.command.Process.Kill()
		return backend.Sandbox{}, ctx.Err()
	}
}
func (b *capacityProcessBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	b.mu.Lock()
	w := b.workers[id]
	b.mu.Unlock()
	if w == nil {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	state := backend.StateRunning
	select {
	case <-w.done:
		state = backend.StateStopped
	default:
	}
	return backend.Sandbox{ID: id, Ref: "controlled-process-" + strconv.Itoa(w.command.Process.Pid), Kind: backend.KindDocker, State: state, Endpoint: "127.0.0.1:49200", CreatedAt: w.started, Instance: "capacity-fixture"}, nil
}
func (b *capacityProcessBackend) List(ctx context.Context) ([]backend.Sandbox, error) {
	b.mu.Lock()
	ids := append([]string(nil), b.order...)
	b.mu.Unlock()
	out := []backend.Sandbox{}
	for _, id := range ids {
		row, err := b.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}
func (b *capacityProcessBackend) Attach(context.Context, string, string) (*websocket.Conn, error) {
	return nil, backend.ErrUnsupported
}
func (b *capacityProcessBackend) Destroy(_ context.Context, id string) error {
	b.mu.Lock()
	w := b.workers[id]
	b.mu.Unlock()
	if w == nil {
		return backend.ErrNotFound
	}
	w.mu.Lock()
	w.destroyRequested = true
	w.mu.Unlock()
	select {
	case <-w.done:
		return nil
	default:
	}
	_ = w.stdin.Close()
	return w.command.Process.Kill()
}
func (b *capacityProcessBackend) Close() error {
	b.mu.Lock()
	workers := make([]*capacityWorker, 0, len(b.workers))
	for _, w := range b.workers {
		workers = append(workers, w)
	}
	b.mu.Unlock()
	for _, w := range workers {
		select {
		case <-w.done:
		default:
			_ = w.stdin.Close()
			_ = w.command.Process.Kill()
			<-w.done
		}
	}
	return nil
}
func (b *capacityProcessBackend) all() []*capacityWorker {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []*capacityWorker{}
	for _, id := range b.order {
		out = append(out, b.workers[id])
	}
	return out
}
func capacityPtr[T any](value T) *T { return &value }

// Admission is deterministic fixture telemetry. It never impersonates the live
// host sampler below, and records requested limits rather than measured RSS.
func (b *capacityProcessBackend) admissionSnapshot() resourcebudget.Snapshot {
	now := time.Now()
	out := resourcebudget.Snapshot{Host: resourcebudget.HostSnapshot{ObservedAt: now, Status: "ready", Scope: "controlled-capacity-fixture", Memory: resourcebudget.Memory{EffectiveTotalBytes: capacityPtr(uint64(32 << 30)), EffectiveAvailableBytes: capacityPtr(uint64(30 << 30))}, CPU: resourcebudget.CPU{EffectiveMillis: capacityPtr(int64(8000))}, Disks: []resourcebudget.Disk{{Role: "projects", ObservedAt: now, AvailableBytes: capacityPtr(uint64(100 << 30))}}}, Execution: resourcebudget.ExecutionSnapshot{ObservedAt: now, Status: "ready", Scope: "controlled-process-fixture", TotalMemoryBytes: capacityPtr(uint64(16 << 30)), CPUMillis: capacityPtr(int64(8000)), InventoryComplete: true, ImageVirtualSizeBytes: capacityPtr(uint64(1 << 20)), DaemonDisk: &resourcebudget.Disk{Role: "execution", ObservedAt: now, AvailableBytes: capacityPtr(uint64(100 << 30))}}}
	for _, w := range b.all() {
		select {
		case <-w.done:
			continue
		default:
		}
		out.Execution.Containers = append(out.Execution.Containers, resourcebudget.ContainerUsage{ID: w.spec.ID, SandboxID: w.spec.ID, Owned: true, Kind: "agent", MemoryLimitBytes: capacityPtr(uint64(w.spec.Resources.MemoryBytes())), CPULimitMillis: capacityPtr(int64(w.spec.Resources.CPUMillis)), ObservedAt: now})
	}
	return out
}

type capacitySample struct {
	Phase         string                      `json:"phase"`
	At            time.Time                   `json:"at"`
	LiveProcesses int                         `json:"liveProcesses"`
	Host          resourcebudget.HostSnapshot `json:"host"`
}
type capacityResult struct {
	Project         string            `json:"project"`
	Task            string            `json:"task"`
	SessionID       string            `json:"sessionId"`
	PID             int               `json:"pid"`
	StartID         string            `json:"startId,omitempty"`
	ExitCode        int               `json:"exitCode"`
	TerminalEvent   string            `json:"terminalEvent"`
	ExpectedFailure bool              `json:"expectedFailure"`
	Resources       backend.Resources `json:"requestedLimits"`
	Started         time.Time         `json:"started"`
	Exited          time.Time         `json:"exited"`
	Successful      bool              `json:"successful"`
	Iterations      int               `json:"iterations"`
	OutputBytes     int64             `json:"outputBytes"`
	ContextDigest   string            `json:"contextDigest"`
	CapturedBytes   int64             `json:"capturedBytes"`
	WorkspaceBytes  int64             `json:"workspaceBytes"`
	UserCPUMS       float64           `json:"userCpuMs"`
	SystemCPUMS     float64           `json:"systemCpuMs"`
}

func TestProjectSwarmFourProjectControlledCapacity(t *testing.T) {
	if os.Getenv("LUCI_SWARM_MULTI_PROJECT") != "1" {
		t.Skip("Set LUCI_SWARM_MULTI_PROJECT=1 for the controlled four-project process acceptance")
	}
	seconds := 3
	if raw := os.Getenv("LUCI_SWARM_MULTI_PROJECT_SECONDS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 1800 {
			t.Fatal("duration must be 1–1800 seconds")
		}
		seconds = value
	}
	report := struct {
		Version                     int                          `json:"version"`
		Passed                      bool                         `json:"passed"`
		Started                     time.Time                    `json:"started"`
		DurationSeconds             int                          `json:"durationSeconds"`
		ReadinessTimeoutMS          int64                        `json:"readinessTimeoutMs"`
		Scope                       string                       `json:"scope"`
		Limits                      []string                     `json:"limits"`
		Policy                      product.SwarmSchedulerPolicy `json:"policy"`
		AdmissionFixture            map[string]any               `json:"admissionFixture"`
		Projects                    []string                     `json:"projects"`
		PeakLiveWorkers             int                          `json:"peakLiveWorkers"`
		RestartRetained             bool                         `json:"restartRetained"`
		AmendmentKeptRunningContext bool                         `json:"amendmentKeptRunningContext"`
		FairFirstWave               bool                         `json:"fairFirstWave"`
		CancelledWithoutAllocation  bool                         `json:"cancelledWithoutAllocation"`
		Results                     []capacityResult             `json:"results"`
		Diagnostics                 []map[string]any             `json:"diagnostics"`
		Samples                     []capacitySample             `json:"samples"`
	}{Version: 1, Started: time.Now(), DurationSeconds: seconds, ReadinessTimeoutMS: backend.ReadyTimeout.Milliseconds(), Scope: "One real API/product service/private FileStore, four reviewed projects, real controlled child processes, actual captured contexts; deterministic admission telemetry is separate from measured host snapshots.", Limits: []string{"No models, Docker enforcement, Windows desktop, deployment, or application result quality claims.", "Restart reopens API/product services and private ledgers in the test process; it is not an OS-process crash or a host reboot.", "Requested worker limits are scheduling fixture inputs; actual touched memory is 16 MiB per process.", "Controlled outputs require human integration and checks; successful process exit is not verified work.", "Host measurements describe this execution host, not a 31.2 GiB Windows workstation."}}
	t.Cleanup(func() {
		report.Passed = !t.Failed()
		if path := os.Getenv("LUCI_SWARM_MULTI_PROJECT_REPORT"); path != "" {
			data, _ := json.MarshalIndent(report, "", "  ")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Error(err)
			} else if err = os.WriteFile(path, append(data, '\n'), 0600); err != nil {
				t.Error(err)
			}
		}
	})
	f := newSwarmSessionFixtureTasks(t)
	projects := []*swarmSessionFixture{f}
	for i := 1; i < 4; i++ {
		projects = append(projects, f.anotherProject(t, fmt.Sprintf("capacity-project-%d", i)))
	}
	for _, p := range projects {
		report.Projects = append(report.Projects, p.owner.ID)
	}
	process := &capacityProcessBackend{store: func() registry.Store { return f.store }, workers: map[string]*capacityWorker{}}
	t.Cleanup(func() { _ = process.Close() })
	t.Cleanup(func() {
		process.mu.Lock()
		report.PeakLiveWorkers = process.peak
		process.mu.Unlock()
		for _, worker := range process.all() {
			worker.mu.Lock()
			entry := map[string]any{"project": worker.session.ProjectID, "task": worker.session.ImplementationContext.TaskIDs[0], "pid": worker.command.Process.Pid, "createError": worker.createError, "destroyRequested": worker.destroyRequested, "lastEvent": worker.event.Event, "iterations": worker.event.Iterations, "unparsedOutput": worker.diagnosticOutput}
			if worker.exitErr != nil {
				entry["exitError"] = worker.exitErr.Error()
			}
			worker.mu.Unlock()
			select {
			case <-worker.done:
				entry["stderr"] = worker.stderr.String()[:min(worker.stderr.Len(), 4096)]
			default:
				entry["stillRunning"] = true
			}
			row, err := f.store.Get(worker.spec.SessionID)
			if err == nil {
				entry["registryStatus"], entry["registryExitReason"] = row.Status, row.ExitReason
			}
			report.Diagnostics = append(report.Diagnostics, entry)
		}
	})
	f.env.api.deps.Product.Close()
	deps := f.env.api.deps
	deps.Backend = process
	// The shared unit fixture uses a 50 ms readiness timeout for stub backends.
	// Actual executable startup must get the production budget; otherwise the
	// derived 100 ms create deadline kills healthy children on a busy host.
	deps.ReadyTimeout = backend.ReadyTimeout
	deps.ResourceSnapshot = process.admissionSnapshot
	pd, err := product.New(f.options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pd.Close() })
	deps.Product = pd
	api, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}
	f.env.api, f.env.mux = api, api.guarded
	sessionsStubReady(t, nil)
	policy := product.SwarmSchedulerPolicy{MaxActive: 3, MaxPerProject: 1, HeadroomPercent: 25, MinHeadroomMiB: 2048, DiskReserveMiB: 512}
	report.Policy = policy
	report.AdmissionFixture = map[string]any{"hostTotalBytes": uint64(32 << 30), "hostAvailableBytes": uint64(30 << 30), "executionTotalBytes": uint64(16 << 30), "hostAndExecutionCPUMillis": 8000, "projectAndDaemonDiskAvailableBytes": uint64(100 << 30), "imageVirtualSizeBytes": uint64(1 << 20), "scope": "Synthetic admission observations only; actual measured host/process observations are in samples. 25% engine memory and CPU headroom leaves room for three requested 4 GiB/2 CPU workers."}
	configure := func(paused bool) {
		t.Helper()
		w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
		var view struct {
			Revision int `json:"revision"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &view) != nil {
			t.Fatal("scheduler read", w.Code, w.Body.String())
		}
		data, _ := json.Marshal(map[string]any{"expectedRevision": view.Revision, "policy": policy, "dispatchPaused": paused})
		w = f.env.do(t, "PUT", "/v1/product/swarm-scheduler", string(data))
		if w.Code != 200 {
			t.Fatal("scheduler policy", w.Code, w.Body.String())
		}
	}
	configure(true)
	queued := map[string][]product.SwarmQueueItem{}
	queueBodies := map[string]string{}
	for _, p := range projects {
		data, _ := json.Marshal(map[string]any{"expectedRevision": p.run.Revision, "idempotencyKey": "capacity-consent", "taskIds": []string{"api", "ui", "join"}, "priority": "normal"})
		queueBodies[p.owner.ID] = string(data)
		w := f.env.do(t, "POST", p.base()+"/"+p.run.ID+"/swarm/queue", string(data))
		var response struct {
			Items []product.SwarmQueueItem `json:"items"`
		}
		if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Items) != 3 {
			t.Fatal("queue reviewed project", w.Code, w.Body.String())
		}
		queued[p.owner.ID] = response.Items
	}
	if len(process.all()) != 0 {
		t.Fatal("paused queue allocated a process")
	}
	// A lost enqueue response recovers exact consent instead of authorizing new work.
	w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/queue", queueBodies[f.owner.ID])
	if w.Code != 200 {
		t.Fatal("lost-response recovery", w.Code, w.Body.String())
	}
	var cancelled product.SwarmQueueItem
	for _, item := range queued[projects[3].owner.ID] {
		if item.TaskID == "ui" {
			cancelled = item
		}
	}
	body, _ := json.Marshal(map[string]int{"expectedRevision": cancelled.Revision})
	w = f.env.do(t, "POST", "/v1/product/projects/"+cancelled.ProjectID+"/swarm-queue/"+cancelled.ID+"/cancel", string(body))
	if w.Code != 200 {
		t.Fatal("cancel queued task", w.Code, w.Body.String())
	}
	queue := func(p *swarmSessionFixture) product.ProjectSwarmQueue {
		t.Helper()
		w := f.env.do(t, "GET", "/v1/product/projects/"+p.owner.ID+"/swarm-queue", "")
		var q product.ProjectSwarmQueue
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &q) != nil {
			t.Fatal("queue status", w.Code, w.Body.String())
		}
		return q
	}
	sweep := func() {
		t.Helper()
		r := reaper.New(reaper.Config{Store: f.store, Backend: process, Instance: "capacity-fixture"})
		if err := r.Sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	pump := func(predicate func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for !predicate() {
			if err := f.env.api.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if time.Now().After(deadline) {
				states := []product.ProjectSwarmQueue{}
				for _, p := range projects {
					states = append(states, queue(p))
				}
				t.Fatalf("scheduler acceptance timed out: %+v", states)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	startWorkload := func(worker *capacityWorker) {
		t.Helper()
		fmt.Fprintln(worker.stdin, "start")
		deadline := time.Now().Add(10 * time.Second)
		for {
			worker.mu.Lock()
			iterations := worker.event.Iterations
			worker.mu.Unlock()
			if iterations >= 5 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("controlled workload made no measurable progress")
			}
			select {
			case <-worker.done:
				t.Fatal("controlled worker exited before workload progress")
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	var hostProbe *resourcebudget.HostProbe
	lastOwned := ""
	sample := func(phase string) {
		owned := []resourcebudget.OwnedProcess{}
		for _, worker := range process.all() {
			select {
			case <-worker.done:
				continue
			default:
			}
			worker.mu.Lock()
			event := worker.event
			worker.mu.Unlock()
			owned = append(owned, resourcebudget.OwnedProcess{PID: event.PID, Role: "controlled-worker", StartID: event.StartID})
		}
		identity, _ := json.Marshal(owned)
		if hostProbe == nil || string(identity) != lastOwned {
			hostProbe = resourcebudget.NewHostProbe(resourcebudget.ProbeOptions{Disks: []resourcebudget.DiskPath{{Role: "projects", Path: f.options.ProjectsDir}, {Role: "state", Path: f.options.StateDir}}, OwnedProcesses: owned})
			lastOwned = string(identity)
		}
		report.Samples = append(report.Samples, capacitySample{Phase: phase, At: time.Now(), LiveProcesses: len(owned), Host: hostProbe.Sample(context.Background())})
	}
	sample("four projects queued; no controlled workers running")
	configure(false)
	pump(func() bool { return len(process.all()) == 3 })
	first := process.all()
	firstProjects := map[string]bool{}
	for _, worker := range first {
		firstProjects[worker.session.ProjectID] = true
		fmt.Fprintln(worker.stdin, "start")
	}
	if len(firstProjects) != 3 {
		t.Fatal("one project monopolized the initial global slots")
	}
	for _, p := range projects {
		if firstProjects[p.owner.ID] {
			continue
		}
		run, err := f.env.api.deps.Product.GetProjectSwarmRun(p.owner.ID, p.run.ID)
		if err != nil || len(run.Operations) != 0 {
			t.Fatal("queued project allocated a checkout before capacity", err)
		}
	}

	for i := 0; i < seconds; i++ {
		sample("three-project mixed workload; fourth queued")
		if report.Samples[len(report.Samples)-1].LiveProcesses != 3 {
			t.Fatal("controlled worker stopped during sustained workload; see startup/exit diagnostics")
		}
		time.Sleep(time.Second)
	}
	configure(true)
	before := len(process.all())
	f.reopen(t)
	if len(process.all()) != before {
		t.Fatal("controller restart replayed a worker")
	}
	for _, worker := range first {
		row, err := f.store.Get(worker.spec.SessionID)
		if err != nil || row.ProjectID != worker.session.ProjectID || row.ProjectRunOperationID != worker.session.ProjectRunOperationID || row.Status == registry.StatusExited {
			t.Fatal("restart lost a live child's exact identity", err)
		}
		data, err := os.ReadFile(filepath.Join(worker.contextRoot, "LUCI.md"))
		digest := sha256.Sum256(data)
		worker.mu.Lock()
		expected := worker.event.ContextDigest
		worker.mu.Unlock()
		if err != nil || hex.EncodeToString(digest[:]) != expected {
			t.Fatal("restart changed frozen context")
		}
	}
	report.RestartRetained = true
	// Release one actual process while paused, then observe its exit through the
	// production reaper. The fourth project gets the next fair scheduling turn.
	fmt.Fprintln(first[0].stdin, "finish")
	select {
	case <-first[0].done:
	case <-time.After(10 * time.Second):
		t.Fatal("worker failed to finish")
	}
	sweep()
	configure(false)
	pump(func() bool { return len(process.all()) >= 4 })
	fourth := process.all()[3]
	if firstProjects[fourth.session.ProjectID] {
		t.Fatal("fourth project starved behind a second attempt from a served project")
	}
	report.FairFirstWave = true
	startWorkload(fourth)
	// A current planning amendment is distinct from bytes already supplied to
	// a running worker. This project has no further authorized independent task.
	amended := projects[3]
	if fourth.session.ProjectID == amended.owner.ID {
		w := f.env.do(t, "GET", "/v1/product/projects/"+amended.owner.ID+"/plan", "")
		var plan product.ProjectPlan
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &plan) != nil {
			t.Fatal("amendment plan read", w.Code)
		}
		for i := range plan.Tasks {
			if plan.Tasks[i].ID == "ui" {
				plan.Tasks[i].Title = "Amend the cancelled interface task"
			}
		}
		body, _ := json.Marshal(map[string]any{"expectedRevision": plan.Revision, "expectedSpecificationRevision": plan.SpecificationRevision, "tasks": plan.Tasks})
		w = f.env.do(t, "POST", "/v1/product/projects/"+amended.owner.ID+"/plan", string(body))
		if w.Code != 200 {
			t.Fatal("planning amendment", w.Code, w.Body.String())
		}
		data, err := os.ReadFile(filepath.Join(fourth.contextRoot, "LUCI.md"))
		digest := sha256.Sum256(data)
		fourth.mu.Lock()
		expected := fourth.event.ContextDigest
		fourth.mu.Unlock()
		if err != nil || hex.EncodeToString(digest[:]) != expected {
			t.Fatal("amendment rewrote a running worker's context")
		}
		report.AmendmentKeptRunningContext = true
	}
	// Complete the remaining authorized independent tasks. One controlled failed
	// process is observed honestly; neither its exit nor any other output is integrated.
	started := map[string]bool{}
	for _, worker := range process.all() {
		started[worker.spec.ID] = true
	}
	expectedFailureID := ""

	for len(process.all()) < 7 {
		for _, worker := range process.all() {
			select {
			case <-worker.done:
				continue
			default:
			}
			if !started[worker.spec.ID] {
				startWorkload(worker)
				started[worker.spec.ID] = true
			}
			command := "finish"
			if expectedFailureID == "" {
				command = "fail"
				expectedFailureID = worker.spec.ID
			}
			fmt.Fprintln(worker.stdin, command)
			select {
			case <-worker.done:
			case <-time.After(10 * time.Second):
				t.Fatal("worker did not acknowledge completion")
			}
		}
		sweep()
		prior := len(process.all())
		pump(func() bool { return len(process.all()) > prior })
	}
	for _, worker := range process.all() {
		select {
		case <-worker.done:
			continue
		default:
		}
		if !started[worker.spec.ID] {
			startWorkload(worker)
		}
		fmt.Fprintln(worker.stdin, "finish")
		select {
		case <-worker.done:
		case <-time.After(10 * time.Second):
			t.Fatal("last worker did not exit")
		}
	}
	sweep()
	_ = f.env.api.deps.Product.RunSwarmSchedulerOnce(context.Background())
	sample("all controlled workers stopped")
	for _, p := range projects {
		for _, item := range queue(p).Items {
			if item.ID == cancelled.ID {
				report.CancelledWithoutAllocation = item.State == "cancelled" && item.OperationID == "" && item.ChildID == ""
			}
			if item.TaskID == "join" && (item.OperationID != "" || item.ChildID != "") {
				t.Fatal("dependency join ran before integrated verified prerequisites")
			}
		}
	}
	process.mu.Lock()
	report.PeakLiveWorkers = process.peak
	process.mu.Unlock()
	if report.PeakLiveWorkers != 3 || !report.CancelledWithoutAllocation {
		t.Fatal("global limit or cancellation contract failed")
	}
	for _, worker := range process.all() {
		worker.mu.Lock()
		event, exitErr, exited, outputBytes := worker.event, worker.exitErr, worker.exited, worker.outputBytes
		worker.mu.Unlock()
		captured, _ := swarmBenchmarkFiles(t, worker.contextRoot)
		workspace, _ := swarmBenchmarkFiles(t, worker.path)
		result := capacityResult{Project: worker.session.ProjectID, Task: worker.session.ImplementationContext.TaskIDs[0], SessionID: worker.spec.ID, PID: worker.command.Process.Pid, StartID: event.StartID, ExitCode: worker.command.ProcessState.ExitCode(), TerminalEvent: event.Event, ExpectedFailure: worker.spec.ID == expectedFailureID, Resources: worker.spec.Resources, Started: worker.started, Exited: exited, Successful: exitErr == nil, Iterations: event.Iterations, OutputBytes: outputBytes, ContextDigest: event.ContextDigest, CapturedBytes: captured, WorkspaceBytes: workspace, UserCPUMS: swarmBenchmarkMS(worker.command.ProcessState.UserTime()), SystemCPUMS: swarmBenchmarkMS(worker.command.ProcessState.SystemTime())}
		report.Results = append(report.Results, result)
		if result.Iterations < 5 {
			t.Fatal("worker finished without the required mixed-workload progress")
		}
		if result.ExpectedFailure {
			if exitErr == nil || result.ExitCode != 2 || result.TerminalEvent != "failed" {
				t.Fatal("deliberate worker failure was not observed with exact exit evidence")
			}
		} else if exitErr != nil || result.ExitCode != 0 || result.TerminalEvent != "completed" {
			t.Fatal("unexpected worker failure", result.Task, result.ExitCode, result.TerminalEvent, exitErr)
		}
		if exitErr == nil {
			data, err := os.ReadFile(filepath.Join(worker.path, result.Task+"-result.txt"))
			var output capacityWorkerEvent
			if err != nil || json.Unmarshal(data, &output) != nil || output.Project != result.Project || output.Task != result.Task || output.ContextDigest != result.ContextDigest {
				t.Fatal("task file does not match its own project context")
			}
		}
		for _, p := range projects {
			if _, err := os.Stat(filepath.Join(p.owner.Path, result.Task+"-result.txt")); !os.IsNotExist(err) {
				t.Fatal("worker result leaked into a primary project")
			}
		}
	}
	sort.Slice(report.Results, func(i, j int) bool { return report.Results[i].Started.Before(report.Results[j].Started) })
	t.Logf("Four shared-service projects; peak %d controlled processes; %d starts, no duplicate after restart; deterministic queue resource inputs and actual host/process samples recorded separately.", report.PeakLiveWorkers, len(report.Results))
}
