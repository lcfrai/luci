package dockerbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type storageReaderSpec struct {
	ConfirmationPath, StateDir, EngineID, Phase string
	Confirmed                                   bool
	Reads                                       int
}
type storageReaderResult struct {
	PID                                                          int
	Phase                                                        string
	Reads, ConfirmedReads, GuestProbeAttempts, HostProbeAttempts int
}

// Invoked only by the parent test. Each child has a separate OS process and
// state directory; it receives only the exact shared host path on stdin.
func TestSharedWindowsStorageReaderProcess(t *testing.T) {
	if os.Getenv("LUCI_STORAGE_READER_PROCESS") != "1" {
		t.Skip("controlled subprocess helper")
	}
	var spec storageReaderSpec
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&spec) != nil || !filepath.IsAbs(spec.StateDir) || !filepath.IsAbs(spec.ConfirmationPath) || spec.Reads < 1 || spec.Reads > 100 {
		t.Fatal("invalid private subprocess fixture")
	}
	api := &storageAPI{info: system.Info{ID: spec.EngineID, OSType: "linux", OperatingSystem: "Docker Desktop", DockerRootDir: "/var/lib/docker"}, version: "Docker Desktop 4.48.0 (207573)"}
	worker := &Backend{api: api, resourceStorageConfirmationPath: spec.ConfirmationPath, resourceStorageReadOnly: true}
	result := storageReaderResult{PID: os.Getpid(), Phase: spec.Phase}
	guest := func(context.Context, system.Info) *resourcebudget.Disk { result.GuestProbeAttempts++; return nil }
	host := func(context.Context, system.Info, *resourcebudget.Disk) *resourcebudget.HostStorageBound {
		result.HostProbeAttempts++
		return nil
	}
	for i := 0; i < spec.Reads; i++ {
		disk, bound := worker.observeStorageForPlatform(context.Background(), api.info, "windows", guest, host)
		confirmed := disk != nil && bound != nil && bound.State == "confirmed"
		result.Reads++
		if confirmed {
			result.ConfirmedReads++
		}
		if confirmed != spec.Confirmed {
			t.Fatalf("phase %s reader result mismatch: confirmed=%v expected=%v", spec.Phase, confirmed, spec.Confirmed)
		}
	}
	if result.GuestProbeAttempts != 0 || result.HostProbeAttempts != 0 || api.infoCalls != 0 || api.versionCalls != 0 {
		t.Fatal("worker attempted a probe instead of sharing the gateway observation")
	}
	data, e := json.Marshal(result)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(spec.StateDir, "result.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestSharedWindowsStorageIsolatedProcesses(t *testing.T) {
	producer, api, record, observed := storageBackendFixture(t)
	writeStorageRecord(t, producer.resourceStorageConfirmationPath, record)
	guestAttempts, hostAttempts, nativeAttempts := 0, 0, 0
	produce := func() {
		t.Helper()
		guest := func(context.Context, system.Info) *resourcebudget.Disk {
			guestAttempts++
			free := uint64(100 << 30)
			return &resourcebudget.Disk{ObservedAt: time.Now().UTC(), AvailableBytes: &free}
		}
		host := func(ctx context.Context, info system.Info, guest *resourcebudget.Disk) *resourcebudget.HostStorageBound {
			hostAttempts++
			return producer.observeHostStorageForPlatform(ctx, info, guest, "windows", func(context.Context, string) (resourcebudget.LocalStorageObservation, error) {
				nativeAttempts++
				out := observed
				out.SettingsSHA256 = record.SettingsSHA256
				out.ObservedAt = time.Now().UTC()
				return out, nil
			})
		}
		_, bound := producer.observeStorageForPlatform(context.Background(), api.info, "windows", guest, host)
		if bound == nil || bound.State != "confirmed" {
			t.Fatalf("controlled producer did not publish a fresh bound: %+v", bound)
		}
	}
	type phaseSummary struct {
		Phase                                                                 string
		Workers, Reads, ConfirmedReads, GuestProbeAttempts, HostProbeAttempts int
	}
	var summaries []phaseSummary
	runReaders := func(phase, engine string, confirmed bool) {
		t.Helper()
		binary, e := os.Executable()
		if e != nil {
			t.Fatal(e)
		}
		specs := make([]storageReaderSpec, 3)
		for i := range specs {
			specs[i] = storageReaderSpec{ConfirmationPath: producer.resourceStorageConfirmationPath, StateDir: t.TempDir(), EngineID: engine, Phase: phase, Confirmed: confirmed, Reads: 20}
		}
		results := make([]storageReaderResult, len(specs))
		problems := make([]error, len(specs))
		var wait sync.WaitGroup
		for i, spec := range specs {
			wait.Add(1)
			go func(i int, spec storageReaderSpec) {
				defer wait.Done()
				data, _ := json.Marshal(spec)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, binary, "-test.run=^TestSharedWindowsStorageReaderProcess$", "-test.count=1")
				// No credentials or parent process environment are copied. Windows
				// loader necessities and private worker temp/state paths are explicit.
				command.Env = []string{"LUCI_STORAGE_READER_PROCESS=1", "GOMAXPROCS=2", "GORACE=atexit_sleep_ms=0", "LUCI_STATE_DIR=" + spec.StateDir, "TMPDIR=" + spec.StateDir, "TMP=" + spec.StateDir, "TEMP=" + spec.StateDir}
				for _, key := range []string{"SYSTEMROOT", "WINDIR", "PATH"} {
					if value, ok := os.LookupEnv(key); ok {
						command.Env = append(command.Env, key+"="+value)
					}
				}
				command.Dir = spec.StateDir
				command.Stdin = bytes.NewReader(data)
				var output bytes.Buffer
				command.Stdout = &output
				command.Stderr = &output
				if e := command.Run(); e != nil {
					problems[i] = errors.New("isolated storage reader failed: " + output.String())
					return
				}
				result, e := os.ReadFile(filepath.Join(spec.StateDir, "result.json"))
				if e != nil {
					problems[i] = e
					return
				}
				problems[i] = json.Unmarshal(result, &results[i])
			}(i, spec)
		}
		wait.Wait()
		seen := map[int]bool{}
		sum := phaseSummary{Phase: phase, Workers: len(specs)}
		for i, result := range results {
			if problems[i] != nil {
				t.Fatal(problems[i])
			}
			if result.PID <= 0 || result.PID == os.Getpid() || seen[result.PID] || result.Phase != phase || result.Reads != 20 {
				t.Fatal("worker process identity or fixture scope was not isolated")
			}
			seen[result.PID] = true
			sum.Reads += result.Reads
			sum.ConfirmedReads += result.ConfirmedReads
			sum.GuestProbeAttempts += result.GuestProbeAttempts
			sum.HostProbeAttempts += result.HostProbeAttempts
		}
		if sum.GuestProbeAttempts != 0 || sum.HostProbeAttempts != 0 {
			t.Fatal("workspace probes were multiplied")
		}
		summaries = append(summaries, sum)
	}
	produce()
	runReaders("fresh-shared-observation", record.EngineID, true)
	cachePath := resourcebudget.LocalStorageObservationPath(producer.resourceStorageConfirmationPath)
	cached, e := resourcebudget.ReadLocalStorageObservation(cachePath)
	if e != nil {
		t.Fatal(e)
	}
	cached.ObservedAt = time.Now().Add(-6 * time.Second)
	if e = resourcebudget.WriteLocalStorageObservation(cachePath, cached); e != nil {
		t.Fatal(e)
	}
	// A new gateway object represents process-local cache loss on restart.
	// Reading the old shared file does not renew its observation timestamps.
	producer = &Backend{api: api, resourceStorageConfirmationPath: producer.resourceStorageConfirmationPath}
	runReaders("restarted-workers-with-expired-file", record.EngineID, false)
	produce()
	runReaders("fresh-after-producer-recovery", record.EngineID, true)
	runReaders("foreign-engine", "other-engine", false)
	record.Revision++
	writeStorageRecord(t, producer.resourceStorageConfirmationPath, record)
	runReaders("changed-confirmation-revision", record.EngineID, false)
	produce()
	record.SettingsSHA256 = strings.Repeat("d", 64)
	writeStorageRecord(t, producer.resourceStorageConfirmationPath, record)
	runReaders("changed-pins-same-revision", record.EngineID, false)
	if guestAttempts != 3 || hostAttempts != 3 || nativeAttempts != 3 {
		t.Fatal("producer invocations were not exactly bounded", guestAttempts, hostAttempts, nativeAttempts)
	}
	report := struct {
		Scope                                                                   string
		ProducerGuestAttempts, ProducerHostAttempts, ProducerNativeHookAttempts int
		Phases                                                                  []phaseSummary
	}{
		"Controlled same-daemon-identity fixture; actual isolated reader subprocesses and private shared files. Producer memory reset uses a new backend object, not an OS gateway restart. Probe hooks count every attempted boundary invocation; no Docker, WSL or native volume probes.",
		guestAttempts, hostAttempts, nativeAttempts, summaries,
	}
	data, _ := json.Marshal(report)
	t.Log(string(data))
}
