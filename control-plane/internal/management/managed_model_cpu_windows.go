//go:build windows

package management

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"golang.org/x/sys/windows"
)

// CPU enforcement is reviewed for this signed Ollama 0.40.8 build and its
// llama.cpp b11738 environment contract, not for arbitrary signed versions.
// The original no-write/no-delete binary pin remains held through retirement.
func managedCPUCompatibleBinary(pin *managedModelPinnedFile) bool {
	return pin != nil && pin.handle != 0 && pin.handle != windows.InvalidHandle && !pin.closed && !pin.closeAttempted && !pin.cleanupUnknown &&
		pin.size == 28141960 && pin.digest == "034a26a71836df581ef8b9ab4db489154f548c81b53aa8816d0e843b9e385c2b"
}

func managedCPUEnvironment(env map[string]string) {
	// In Ollama v0.40.8, SetupLlamaServerCommandEnv preserves LLAMA_ARG_*
	// from the owned daemon. GetDevicesEnv can rewrite visibility masks, but
	// not these three controls; its only persisted runner override is Metal's
	// GGML_METAL_TENSOR_DISABLE. DEVICE=none is an independent offload fence.
	// NumGPU=-1 in the retained alias emits no CLI -ngl override. No model or
	// global setting is rewritten to select this mode.
	env["LLAMA_ARG_DEVICE"] = "none"
	env["LLAMA_ARG_N_GPU_LAYERS"] = "0"
	env["LLAMA_ARG_MMPROJ_OFFLOAD"] = "0"
	env["CUDA_VISIBLE_DEVICES"] = "-1"
	env["HIP_VISIBLE_DEVICES"] = "-1"
	env["ROCR_VISIBLE_DEVICES"] = "-1"
	env["OLLAMA_VULKAN"] = "0"
	env["GGML_VK_VISIBLE_DEVICES"] = "-1"
	env["OLLAMA_KV_CACHE_TYPE"] = "f16"
	env["OLLAMA_FLASH_ATTENTION"] = "0"
}

// This is the first metadata request in CPU mode. It never loads a model, and
// the caller's existing owned-listener/connected-peer transport and deadline
// apply. A valid binary pin alone is not a successful runtime version report.
func managedCPUVersion(ctx context.Context, metadata func(context.Context, string, string, any) error) error {
	var version struct {
		Version *string `json:"version"`
	}
	if err := metadata(ctx, "/api/version", "", &version); err != nil {
		return err
	}
	if version.Version == nil || *version.Version != "0.40.8" {
		return errors.New("CPU runtime version is unsupported")
	}
	return nil
}

func (s *windowsManagedModelService) cpuVersionFailure(result managedModelSample, err error) (managedModelSample, error) {
	// Preserve exact-owner refusals and the original startup wait for transient
	// transport failures. Semantic/version failures never grant CPU compatibility.
	if detail := managedObservationDiagnostic(err); detail != nil {
		if detail.Stage == "listener" || detail.Stage == "peer" || detail.Reason == "timeout" || detail.Reason == "connect_refused" || detail.Reason == "connection_lost" {
			return s.metadataFailure(result, err)
		}
	}
	result.failure = "cpu_runtime_unsupported"
	return result, err
}

type managedLoadedModel struct {
	Name    string  `json:"name"`
	Digest  string  `json:"digest"`
	Context int     `json:"context_length"`
	VRAM    *uint64 `json:"size_vram"`
}

func managedLoadedModelMatches(c managedModelConfig, digest string, row managedLoadedModel) bool {
	mode, err := managedmodel.NormalizeExecutionMode(c.ExecutionMode)
	profile, profileErr := managedmodel.ProfileForExecutionMode(mode)
	if err != nil || profileErr != nil || c.Profile != profile || row.Name != c.Model || row.Digest != digest || row.Context != profile.ContextTokens || row.VRAM == nil {
		return false
	}
	if mode == managedmodel.ExecutionModeCPU {
		return *row.VRAM == 0
	}
	return *row.VRAM > 0
}
