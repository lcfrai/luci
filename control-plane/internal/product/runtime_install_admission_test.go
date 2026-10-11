package product

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func TestRuntimeInstallAdmissionReceiptSurvivesReadsAndRestartWithoutRetry(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	now := time.Now().UTC()
	seen := runtimeinstall.AdmissionObservation{Stage: "initial", ReasonCode: "resource_observation_unavailable", DecidedAt: now, SamplerAvailable: true, HostObservedAt: now.Add(-time.Minute), ExecutionObservedAt: now, ExecutionFresh: true, HostMemoryKnown: true, HostCPUKnown: true, ExecutionMemoryKnown: true, ExecutionCPUKnown: true, InventoryComplete: true, EngineMatched: true}
	s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(_ context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		out := runtimeinstall.Outcome{Identity: req.Identity, Phase: "blocked", ReasonCode: seen.ReasonCode, Message: "Original unavailable sample.", Admission: &seen}
		return out, publish(out)
	}})
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	job := awaitRuntimeStatus(t, mux, "blocked")
	s.Close()
	path := filepath.Join(s.stateDir, "runtime-install.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved RuntimeJob
	if err := json.Unmarshal(before, &saved); err != nil || saved.Execution == nil || saved.Execution.Admission == nil || *saved.Execution.Admission != seen {
		t.Fatal(saved, err)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), RuntimeDir: s.runtimeDir})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.SetRuntimeResourceSnapshot(setupStorageFixtureSnapshot)
	restarted.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(context.Context, runtimeinstall.Request, runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		t.Error("reading retained refusal started another installer")
		return runtimeinstall.Outcome{}, nil
	}})
	next := http.NewServeMux()
	restarted.registerSetupTools(next)
	for range 3 {
		current := readRuntimeJob(t, next)
		if current.ID != job.ID || current.Status != "blocked" || current.Execution == nil || current.Execution.ResourcesHeld || current.Execution.Admission == nil || *current.Execution.Admission != seen {
			t.Fatal("original decision changed after restart/read", current)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("status reads rewrote admission evidence", err)
	}
}
