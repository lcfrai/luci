//go:build windows

package management

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"golang.org/x/sys/windows"
)

// Pure fixtures only: no installed file, process, Job, GPU or network access.
func TestManagedCPUCompatibilityRequiresExactRetainedReviewedBuild(t *testing.T) {
	good := managedModelPinnedFile{handle: 55, size: 28141960, digest: "034a26a71836df581ef8b9ab4db489154f548c81b53aa8816d0e843b9e385c2b"}
	for _, tc := range []struct {
		name   string
		change func(*managedModelPinnedFile)
		allow  bool
	}{
		{"reviewed", func(*managedModelPinnedFile) {}, true},
		{"different_binary", func(p *managedModelPinnedFile) { p.digest = strings.Repeat("a", 64) }, false},
		{"different_size", func(p *managedModelPinnedFile) { p.size++ }, false},
		{"no_original", func(p *managedModelPinnedFile) { p.handle = 0 }, false},
		{"invalid_original", func(p *managedModelPinnedFile) { p.handle = windows.InvalidHandle }, false},
		{"closed", func(p *managedModelPinnedFile) { p.closed = true }, false},
		{"close_attempted", func(p *managedModelPinnedFile) { p.closeAttempted = true }, false},
		{"cleanup_unknown", func(p *managedModelPinnedFile) { p.cleanupUnknown = true }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pin := good
			tc.change(&pin)
			if managedCPUCompatibleBinary(&pin) != tc.allow {
				t.Fatal("unknown or unretained build gained CPU launch compatibility")
			}
		})
	}
	if managedCPUCompatibleBinary(nil) {
		t.Fatal("missing binary gained CPU compatibility")
	}
}

func TestManagedCPUOwnedEnvironmentDisablesOffloadWithoutChangingModel(t *testing.T) {
	env := map[string]string{"OLLAMA_MODELS": `E:\InstalledModels`, "OLLAMA_HOST": "127.0.0.1:11435", "OLLAMA_NO_CLOUD": "true", "OLLAMA_NUM_PARALLEL": "1", "OLLAMA_MAX_LOADED_MODELS": "1", "CUDA_VISIBLE_DEVICES": "0", "LLAMA_ARG_DEVICE": "CUDA0", "LLAMA_ARG_N_GPU_LAYERS": "99", "LLAMA_ARG_MMPROJ_OFFLOAD": "1", "OLLAMA_KV_CACHE_TYPE": "q8_0", "OLLAMA_FLASH_ATTENTION": "1"}
	managedCPUEnvironment(env)
	for key, want := range map[string]string{"LLAMA_ARG_DEVICE": "none", "LLAMA_ARG_N_GPU_LAYERS": "0", "LLAMA_ARG_MMPROJ_OFFLOAD": "0", "CUDA_VISIBLE_DEVICES": "-1", "HIP_VISIBLE_DEVICES": "-1", "ROCR_VISIBLE_DEVICES": "-1", "OLLAMA_VULKAN": "0", "GGML_VK_VISIBLE_DEVICES": "-1", "OLLAMA_KV_CACHE_TYPE": "f16", "OLLAMA_FLASH_ATTENTION": "0"} {
		if env[key] != want {
			t.Fatalf("CPU offload/cache control %s was not enforced", key)
		}
	}
	if env["OLLAMA_MODELS"] != `E:\InstalledModels` || env["OLLAMA_HOST"] != "127.0.0.1:11435" || env["OLLAMA_NO_CLOUD"] != "true" || env["OLLAMA_NUM_PARALLEL"] != "1" || env["OLLAMA_MAX_LOADED_MODELS"] != "1" {
		t.Fatal("CPU selection changed model storage, endpoint or local-only concurrency")
	}
}

func TestManagedCPURuntimeVersionIsOwnedMetadataBeforeOtherRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		owned     bool
		allow     bool
	}{
		{"supported", `{"version":"0.40.8"}`, true, true},
		{"other_version", `{"version":"0.40.9"}`, true, false},
		{"missing", `{}`, true, false},
		{"null", `{"version":null}`, true, false},
		{"numeric", `{"version":408}`, true, false},
		{"invalid", `private malformed response`, true, false},
		{"foreign_listener", `{"version":"0.40.8"}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deadline := time.Now().Add(time.Second)
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()
			body := &managedObservationBody{Reader: strings.NewReader(tc.raw)}
			requests, ownershipChecks := 0, 0
			client := &http.Client{Transport: managedObservationRoundTrip(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Method != "GET" || r.URL.Path != "/api/version" || r.Body != nil {
					t.Fatal("version gate sent a model request")
				}
				if got, _ := r.Context().Deadline(); !got.Equal(deadline) {
					t.Fatal("version gate replaced the caller deadline")
				}
				return &http.Response{StatusCode: 200, Body: body}, nil
			})}
			err := managedCPUVersion(ctx, func(ctx context.Context, path, input string, out any) error {
				return managedModelMetadata(ctx, client, "http://127.0.0.1:11435", path, input, out, func() error {
					ownershipChecks++
					if !tc.owned {
						return errors.New("private ownership detail")
					}
					return nil
				})
			})
			if (err == nil) != tc.allow || requests != 1 || ownershipChecks != 1 || body.closed != 1 {
				t.Fatal("runtime compatibility used an unsupported/unowned report or lost response settlement", err, requests, ownershipChecks, body.closed)
			}
		})
	}
	service := windowsManagedModelService{config: managedModelConfig{ExecutionMode: "cpu"}}
	for _, path := range []string{"/api/tags", "/api/show", "/api/ps", "/api/generate"} {
		if err := service.metadata(context.Background(), path, "", &struct{}{}); err == nil {
			t.Fatal("metadata/model route ran before CPU compatibility", path)
		}
	}
}

func TestManagedLoadedProfileRequiresExplicitCPUZeroAndPreservesGPU(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, mode, vram string
		model            string
		context          int
		allow            bool
	}{
		{"cpu_zero", "cpu", `,"size_vram":0`, "installed:latest", 16384, true},
		{"cpu_gpu_allocated", "cpu", `,"size_vram":1`, "installed:latest", 16384, false},
		{"cpu_missing", "cpu", ``, "installed:latest", 16384, false},
		{"cpu_null", "cpu", `,"size_vram":null`, "installed:latest", 16384, false},
		{"cpu_wrong_model", "cpu", `,"size_vram":0`, "other:latest", 16384, false},
		{"cpu_wrong_context", "cpu", `,"size_vram":0`, "installed:latest", 4096, false},
		{"gpu_allocated", "gpu", `,"size_vram":1`, "installed:latest", 16384, true},
		{"legacy_gpu_allocated", "", `,"size_vram":1`, "installed:latest", 16384, true},
		{"gpu_cpu_only", "gpu", `,"size_vram":0`, "installed:latest", 16384, false},
		{"gpu_null", "gpu", `,"size_vram":null`, "installed:latest", 16384, false},
		{"unknown_mode", "auto", `,"size_vram":0`, "installed:latest", 16384, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile, _ := managedmodel.ProfileForExecutionMode(tc.mode)
			config := managedModelConfig{Model: "installed:latest", ExecutionMode: tc.mode, Profile: profile}
			raw, _ := json.Marshal(map[string]any{"name": tc.model, "digest": digest, "context_length": tc.context})
			raw = append(raw[:len(raw)-1], []byte(tc.vram+"}")...)
			var row managedLoadedModel
			if err := json.Unmarshal(raw, &row); err != nil || managedLoadedModelMatches(config, digest, row) != tc.allow {
				t.Fatal("loaded profile accepted unknown CPU evidence or changed GPU admission", err)
			}
		})
	}
	for _, raw := range []string{`{"size_vram":-1}`, `{"size_vram":1.5}`, `{"size_vram":"0"}`} {
		var row managedLoadedModel
		if json.Unmarshal([]byte(raw), &row) == nil {
			t.Fatal("invalid VRAM value became CPU evidence")
		}
	}
}

func TestManagedCPUVersionFailuresRetainOriginalStartupDeadline(t *testing.T) {
	for _, tc := range []struct {
		name, stage, reason, want string
		expired                   bool
		wait                      bool
	}{
		{"timeout_wait", "version", "timeout", "starting", false, true},
		{"refused_wait", "version", "connect_refused", "starting", false, true},
		{"reset_wait", "version", "connection_lost", "starting", false, true},
		{"original_deadline", "version", "timeout", "startup_timeout", true, false},
		{"invalid_response", "version", "response_invalid", "cpu_runtime_unsupported", false, false},
		{"http_status", "version", "http_status", "cpu_runtime_unsupported", false, false},
		{"peer_refusal", "peer", "owner_not_in_job", "listener_ownership_unavailable", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deadline := time.Now().Add(time.Minute)
			if tc.expired {
				deadline = time.Now().Add(-time.Second)
			}
			service := windowsManagedModelService{config: managedModelConfig{ExecutionMode: "cpu"}, startupDeadline: deadline}
			err := managedObservationFailure(tc.stage, tc.reason, errors.New("private runtime detail"), 0)
			if detail := managedObservationDiagnostic(err); detail == nil || detail.Stage != tc.stage {
				t.Fatal("fixed version/owner diagnostic was dropped")
			}
			result, err := service.cpuVersionFailure(managedModelSample{}, err)
			if result.failure != tc.want || (err == nil) != tc.wait || !service.startupDeadline.Equal(deadline) || service.cpuVersionVerified || service.metadataReady {
				t.Fatal("CPU version failure gained readiness or a new startup budget", result.failure, err)
			}
		})
	}
	service := windowsManagedModelService{}
	result, err := service.cpuVersionFailure(managedModelSample{}, errors.New("CPU runtime version is unsupported"))
	if err == nil || result.failure != "cpu_runtime_unsupported" || service.cpuVersionVerified {
		t.Fatal("mismatched version became a transient or CPU capability")
	}
}
