package api

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const swarmBenchmarkRounds = 1_000_000

type swarmBenchmarkResult struct {
	Task   string `json:"task"`
	Rounds int    `json:"rounds"`
	Digest string `json:"digest"`
}
type swarmBenchmarkWorkerEvent struct {
	Event  string                `json:"event"`
	At     string                `json:"at"`
	PID    int                   `json:"pid"`
	Result *swarmBenchmarkResult `json:"result,omitempty"`
}

// This worker is an explicitly controlled CPU/file workload, not an AI agent.
// Parent barriers separate admission latency, ready-process overlap and compute.
func TestSwarmBenchmarkWorkerProcess(t *testing.T) {
	if os.Getenv("LUCI_SWARM_BENCHMARK_PROCESS") != "1" {
		return
	}
	task, contextRoot := os.Getenv("LUCI_SWARM_BENCHMARK_TASK"), os.Getenv("LUCI_SWARM_BENCHMARK_CONTEXT")
	data, err := os.ReadFile(filepath.Join(contextRoot, "LUCI.md"))
	if err != nil {
		t.Fatal(err)
	}
	marker := bytes.Index(data, []byte("\n```json\n"))
	if marker < 0 {
		t.Fatal("missing captured task contract")
	}
	var brief struct {
		SelectedTasks []struct {
			ID         string   `json:"id"`
			WritePaths []string `json:"writePaths"`
		} `json:"selectedTasks"`
	}
	if err = json.NewDecoder(bytes.NewReader(data[marker+len("\n```json\n"):])).Decode(&brief); err != nil || len(brief.SelectedTasks) != 1 || brief.SelectedTasks[0].ID != task || len(brief.SelectedTasks[0].WritePaths) != 1 || brief.SelectedTasks[0].WritePaths[0] != task+"-result.txt" {
		t.Fatal("worker assignment differs from its captured task", err)
	}
	emit := func(event string, result *swarmBenchmarkResult) {
		if err := json.NewEncoder(os.Stdout).Encode(swarmBenchmarkWorkerEvent{Event: event, At: time.Now().UTC().Format(time.RFC3339Nano), PID: os.Getpid(), Result: result}); err != nil {
			t.Fatal(err)
		}
	}
	input := bufio.NewScanner(os.Stdin)
	emit("ready", nil)
	if !input.Scan() || input.Text() != "compute" {
		t.Fatal("missing explicit computation release")
	}
	emit("compute-start", nil)
	digest := sha256.Sum256([]byte("Luci controlled swarm benchmark: " + task))
	for range swarmBenchmarkRounds {
		digest = sha256.Sum256(digest[:])
	}
	result := swarmBenchmarkResult{Task: task, Rounds: swarmBenchmarkRounds, Digest: hex.EncodeToString(digest[:])}
	output, _ := json.Marshal(result)
	if err = os.WriteFile(task+"-result.txt", append(output, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	emit("compute-done", &result)
	// Keep the completed process observable long enough to read OS high-water
	// RSS. This wait is excluded from its compute duration and billed as nothing.
	if !input.Scan() || input.Text() != "exit" {
		t.Fatal("missing exit acknowledgement")
	}
	os.Exit(0)
}

type swarmBenchmarkTask struct {
	Task              string               `json:"task"`
	SessionID         string               `json:"sessionId"`
	OperationID       string               `json:"operationId"`
	WorktreeID        string               `json:"worktreeId"`
	PID               int                  `json:"pid"`
	DispatchMS        float64              `json:"dispatchMs"`
	WorkerReadyAt     string               `json:"workerReadyAt"`
	ComputeStartedAt  string               `json:"computeStartedAt"`
	ComputeFinishedAt string               `json:"computeFinishedAt"`
	ProcessExitedAt   string               `json:"processExitedAt"`
	ComputeMS         float64              `json:"computeMs"`
	UserCPUMS         float64              `json:"userCpuMs"`
	SystemCPUMS       float64              `json:"systemCpuMs"`
	PeakRSSBytes      *int64               `json:"peakRssBytes"`
	WorkspaceBytes    int64                `json:"workspaceBytes"`
	WorkspaceFiles    int                  `json:"workspaceFiles"`
	CapturedBytes     int64                `json:"capturedBytes"`
	CapturedFiles     int                  `json:"capturedFiles"`
	Result            swarmBenchmarkResult `json:"result"`
}
type swarmBenchmarkTrial struct {
	Mode                  string                `json:"mode"`
	Trial                 int                   `json:"trial"`
	FixtureSetupMS        float64               `json:"fixtureSetupMs"`
	ControllerStages      []swarmBenchmarkStage `json:"controllerStages"`
	DispatchAndResultsMS  float64               `json:"dispatchAndResultsMs"`
	PeakLiveWorkers       int                   `json:"peakLiveWorkers"`
	ReadyProcessOverlapMS float64               `json:"readyProcessOverlapMs"`
	ComputeOverlapMS      float64               `json:"computeOverlapMs"`
	Tasks                 []swarmBenchmarkTask  `json:"tasks"`
}
type swarmBenchmarkStage struct {
	Stage     string  `json:"stage"`
	Task      string  `json:"task,omitempty"`
	ElapsedMS float64 `json:"elapsedMs"`
}
type swarmBenchmarkWorker struct {
	cmd               *exec.Cmd
	input             io.WriteCloser
	events            <-chan swarmBenchmarkWorkerEvent
	stderr            *bytes.Buffer
	path, contextRoot string
	record            swarmBenchmarkTask
	exited            bool
}

func swarmBenchmarkMS(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
func swarmBenchmarkEvent(t *testing.T, worker *swarmBenchmarkWorker, expected string) swarmBenchmarkWorkerEvent {
	t.Helper()
	select {
	case event, open := <-worker.events:
		if !open || event.Event != expected || event.PID != worker.cmd.Process.Pid {
			t.Fatalf("worker event: expected %s; received %+v; stderr %s", expected, event, worker.stderr.String())
		}
		return event
	case <-time.After(20 * time.Second):
		t.Fatalf("worker timed out waiting for %s", expected)
	}
	return swarmBenchmarkWorkerEvent{}
}
func swarmBenchmarkFiles(t *testing.T, root string) (int64, int) {
	t.Helper()
	var size int64
	files := 0
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			size += info.Size()
			files++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return size, files
}
func swarmBenchmarkPeakRSS(pid int) *int64 {
	if runtime.GOOS != "linux" {
		return nil
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "VmHWM:" && fields[2] == "kB" {
			value, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return nil
			}
			value *= 1024
			return &value
		}
	}
	return nil
}
func swarmBenchmarkOverlap(aStart, aEnd, bStart, bEnd string) float64 {
	a, _ := time.Parse(time.RFC3339Nano, aStart)
	b, _ := time.Parse(time.RFC3339Nano, bStart)
	x, _ := time.Parse(time.RFC3339Nano, aEnd)
	y, _ := time.Parse(time.RFC3339Nano, bEnd)
	if b.After(a) {
		a = b
	}
	if y.Before(x) {
		x = y
	}
	if !x.After(a) {
		return 0
	}
	return swarmBenchmarkMS(x.Sub(a))
}

func runSwarmBenchmarkTrial(t *testing.T, mode string, trial int) swarmBenchmarkTrial {
	t.Helper()
	setupStarted := time.Now()
	f := newSwarmSessionFixture(t)
	report := swarmBenchmarkTrial{Mode: mode, Trial: trial, FixtureSetupMS: swarmBenchmarkMS(time.Since(setupStarted)), ControllerStages: []swarmBenchmarkStage{}, Tasks: []swarmBenchmarkTask{}}
	for _, stage := range f.timings {
		report.ControllerStages = append(report.ControllerStages, swarmBenchmarkStage{Stage: stage.Stage, Task: stage.Task, ElapsedMS: swarmBenchmarkMS(stage.Elapsed)})
	}
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	workers := map[string]*swarmBenchmarkWorker{}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.env.be.onCreate = func(spec backend.Spec) {
		child, err := f.store.Get(spec.SessionID)
		if err != nil || child.ImplementationContext == nil || len(child.ImplementationContext.TaskIDs) != 1 {
			t.Error("worker ownership was not persisted", err)
			return
		}
		worker := &swarmBenchmarkWorker{stderr: &bytes.Buffer{}, record: swarmBenchmarkTask{Task: child.ImplementationContext.TaskIDs[0], SessionID: child.ID, OperationID: child.ProjectRunOperationID, WorktreeID: child.WorktreeID}}
		for _, mount := range spec.Mounts {
			if mount.ContainerPath == sessionWorkDir {
				worker.path = mount.HostPath
			}
			if mount.ContainerPath == reviewedContextContainerDir && mount.ReadOnly {
				worker.contextRoot = mount.HostPath
			}
		}
		if worker.path == "" || worker.contextRoot == "" || worker.path == f.owner.Path {
			t.Error("worker lacks isolated source and captured context")
			return
		}
		worker.cmd = exec.Command(executable, "-test.run=^TestSwarmBenchmarkWorkerProcess$")
		worker.cmd.Dir = worker.path
		worker.cmd.Env = append(os.Environ(), "LUCI_SWARM_BENCHMARK_PROCESS=1", "LUCI_SWARM_BENCHMARK_TASK="+worker.record.Task, "LUCI_SWARM_BENCHMARK_CONTEXT="+worker.contextRoot)
		worker.cmd.Stderr = worker.stderr
		worker.input, err = worker.cmd.StdinPipe()
		if err != nil {
			t.Error(err)
			return
		}
		output, err := worker.cmd.StdoutPipe()
		if err != nil {
			t.Error(err)
			return
		}
		if err = worker.cmd.Start(); err != nil {
			t.Error(err)
			return
		}
		workers[child.ID] = worker
		worker.record.PID = worker.cmd.Process.Pid
		events := make(chan swarmBenchmarkWorkerEvent, 3)
		worker.events = events
		go func() {
			defer close(events)
			decoder := json.NewDecoder(output)
			for {
				var event swarmBenchmarkWorkerEvent
				if decoder.Decode(&event) != nil {
					return
				}
				events <- event
			}
		}()
		worker.record.WorkerReadyAt = swarmBenchmarkEvent(t, worker, "ready").At
	}
	t.Cleanup(func() {
		for _, worker := range workers {
			if !worker.exited {
				_ = worker.input.Close()
				_ = worker.cmd.Process.Kill()
				_ = worker.cmd.Wait()
			}
		}
	})
	start := func(index int) *swarmBenchmarkWorker {
		body, _ := json.Marshal(f.requests[index])
		started := time.Now()
		created := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
		sessionsAwaitWatch(t, done)
		worker := workers[created.ID]
		if worker == nil {
			t.Fatal("API did not start controlled worker")
		}
		worker.record.DispatchMS = swarmBenchmarkMS(time.Since(started))
		return worker
	}
	release := func(worker *swarmBenchmarkWorker) {
		if _, err := fmt.Fprintln(worker.input, "compute"); err != nil {
			t.Fatal(err)
		}
	}
	finish := func(worker *swarmBenchmarkWorker) {
		worker.record.ComputeStartedAt = swarmBenchmarkEvent(t, worker, "compute-start").At
		completed := swarmBenchmarkEvent(t, worker, "compute-done")
		if completed.Result == nil || completed.Result.Task != worker.record.Task || completed.Result.Rounds != swarmBenchmarkRounds {
			t.Fatal("incomplete controlled result")
		}
		worker.record.ComputeFinishedAt, worker.record.Result = completed.At, *completed.Result
		started, _ := time.Parse(time.RFC3339Nano, worker.record.ComputeStartedAt)
		ended, _ := time.Parse(time.RFC3339Nano, completed.At)
		worker.record.ComputeMS = swarmBenchmarkMS(ended.Sub(started))
		worker.record.PeakRSSBytes = swarmBenchmarkPeakRSS(worker.record.PID)
		worker.record.WorkspaceBytes, worker.record.WorkspaceFiles = swarmBenchmarkFiles(t, worker.path)
		worker.record.CapturedBytes, worker.record.CapturedFiles = swarmBenchmarkFiles(t, worker.contextRoot)
		data, err := os.ReadFile(filepath.Join(worker.path, worker.record.Task+"-result.txt"))
		if err != nil {
			t.Fatal(err)
		}
		var result swarmBenchmarkResult
		if json.Unmarshal(data, &result) != nil || result != worker.record.Result {
			t.Fatal("reported result differs from the actual task file")
		}
		if _, err := os.Stat(filepath.Join(f.owner.Path, worker.record.Task+"-result.txt")); !os.IsNotExist(err) {
			t.Fatal("worker result leaked into main before review")
		}
		if _, err := fmt.Fprintln(worker.input, "exit"); err != nil {
			t.Fatal(err)
		}
		_ = worker.input.Close()
		if err := worker.cmd.Wait(); err != nil {
			t.Fatal("controlled worker exit", err, worker.stderr.String())
		}
		worker.exited = true
		worker.record.ProcessExitedAt = time.Now().UTC().Format(time.RFC3339Nano)
		worker.record.UserCPUMS = swarmBenchmarkMS(worker.cmd.ProcessState.UserTime())
		worker.record.SystemCPUMS = swarmBenchmarkMS(worker.cmd.ProcessState.SystemTime())
		report.Tasks = append(report.Tasks, worker.record)
	}
	started := time.Now()
	if mode == "sequential" {
		for i := range f.requests {
			worker := start(i)
			release(worker)
			finish(worker)
		}
		report.PeakLiveWorkers = 1
	} else {
		ordered := []*swarmBenchmarkWorker{}
		for i := range f.requests {
			ordered = append(ordered, start(i))
		}
		for _, worker := range ordered {
			release(worker)
		}
		for _, worker := range ordered {
			finish(worker)
		}
		report.PeakLiveWorkers = len(ordered)
	}
	report.DispatchAndResultsMS = swarmBenchmarkMS(time.Since(started))
	if len(report.Tasks) != 2 || len(f.env.be.specs) != 2 {
		t.Fatal("benchmark allocated unexpected workers")
	}
	a, b := report.Tasks[0], report.Tasks[1]
	report.ReadyProcessOverlapMS = swarmBenchmarkOverlap(a.WorkerReadyAt, a.ProcessExitedAt, b.WorkerReadyAt, b.ProcessExitedAt)
	report.ComputeOverlapMS = swarmBenchmarkOverlap(a.ComputeStartedAt, a.ComputeFinishedAt, b.ComputeStartedAt, b.ComputeFinishedAt)
	if mode == "parallel" && report.ReadyProcessOverlapMS <= 0 || mode == "sequential" && report.ReadyProcessOverlapMS != 0 {
		t.Fatal("observed process overlap differs from benchmark mode")
	}
	return report
}

// Opt in to repeated measurement. Ordinary correctness suites do not spend
// worker CPU on timing samples or accidentally turn timing into a release gate.
func TestProjectSwarmControlledBenchmark(t *testing.T) {
	if os.Getenv("LUCI_SWARM_BENCHMARK") != "1" {
		t.Skip("Set LUCI_SWARM_BENCHMARK=1 for the controlled local workload comparison")
	}
	report := struct {
		Version          int                   `json:"version"`
		MeasuredAt       string                `json:"measuredAt"`
		OS               string                `json:"os"`
		Arch             string                `json:"arch"`
		LogicalCPUs      int                   `json:"logicalCpus"`
		GOMAXPROCS       int                   `json:"gomaxprocs"`
		Workload         string                `json:"workload"`
		ResultEquivalent bool                  `json:"resultEquivalent"`
		Limits           []string              `json:"limits"`
		Trials           []swarmBenchmarkTrial `json:"trials"`
	}{Version: 1, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano), OS: runtime.GOOS, Arch: runtime.GOARCH, LogicalCPUs: runtime.NumCPU(), GOMAXPROCS: runtime.GOMAXPROCS(0),
		Workload: "Two complementary controlled processes each read their singleton captured task contract, perform 1,000,000 chained SHA-256 operations and write one deterministic assigned result file.",
		Limits:   []string{"Real local test-binary processes and Git copies; lifecycle backend double, no Docker or model inference.", "Both modes prepare the same two reservations with maxParallel=2; sequential mode waits for each process exit before starting the next.", "Parallel workers wait at an explicit start barrier until both are ready; readiness overlap includes that barrier. Compute overlap is reported separately.", "Dispatch duration includes HTTP admission, backend process startup and the stubbed readiness watcher. Initial project/review/Git setup is reported separately.", "Results are controlled task files, not independently verified integrated application behavior. No merge, deployment or paid-service acceptance is measured.", "Workspace bytes include regular files and Git metadata after result creation; captured bytes include eight documents and manifest, not provider token use.", "Linux peak RSS is per-process VmHWM observed after computation; null on unavailable platforms. CPU times are OS-reported for each completed child. No aggregate host resource peak is inferred.", "No provider tokens or cost observed. No coding-agent, competitor or production throughput/speedup claim is supported by this fixture."}, Trials: []swarmBenchmarkTrial{}}
	results := map[string]swarmBenchmarkResult{}
	for trial := 1; trial <= 3; trial++ {
		modes := []string{"sequential", "parallel"}
		if trial%2 == 0 {
			modes = []string{"parallel", "sequential"}
		}
		for _, mode := range modes {
			var measured swarmBenchmarkTrial
			if !t.Run(fmt.Sprintf("trial-%d-%s", trial, mode), func(t *testing.T) { measured = runSwarmBenchmarkTrial(t, mode, trial) }) {
				return
			}
			for _, task := range measured.Tasks {
				if prior, ok := results[task.Task]; ok && prior != task.Result {
					t.Fatal("serial/parallel controlled result mismatch", task.Task)
				}
				results[task.Task] = task.Result
			}
			report.Trials = append(report.Trials, measured)
		}
	}
	report.ResultEquivalent = len(results) == 2
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("LUCI_SWARM_BENCHMARK_REPORT"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, trial := range report.Trials {
		t.Logf("%s trial%d: dispatch/results %.2fms, peak workers%d, ready overlap %.2fms, compute overlap %.2fms", trial.Mode, trial.Trial, trial.DispatchAndResultsMS, trial.PeakLiveWorkers, trial.ReadyProcessOverlapMS, trial.ComputeOverlapMS)
	}
}
