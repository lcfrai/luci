package deploymentplan

import "fmt"

// Managed GPU nodes are an explicit Azure preview choice. The controller never
// registers the provider feature or substitutes a larger pool automatically.
const AzureManagedGPUAPIVersion = "2026-08-01"
const AzureManagedGPUFeature = "ManagedGPUExperiencePreview"

type AzureGPUNodePoolSpec struct {
	NodeSKU                   string `json:"nodeSku"`
	NodeCount                 int    `json:"nodeCount"`
	AcknowledgeManagedPreview bool   `json:"acknowledgeManagedPreview"`
}
type AzureGPUCapacity struct {
	MilliCPU        int `json:"milliCpu"`
	MemoryMiB       int `json:"memoryMiB"`
	GPUs            int `json:"gpus"`
	MemoryPerGPUMiB int `json:"memoryPerGpuMiB"`
}

func AzureClusterAPI(spec AzureClusterSpec) string {
	if spec.GPUPool != nil {
		return AzureManagedGPUAPIVersion
	}
	return AzureManagedClusterAPIVersion
}
func AzureGPUPoolCapacity(spec *AzureGPUNodePoolSpec) (AzureGPUCapacity, error) {
	if spec == nil || !spec.AcknowledgeManagedPreview || spec.NodeCount < 1 || spec.NodeCount > 4 {
		return AzureGPUCapacity{}, fmt.Errorf("choose a fixed GPU pool and explicitly acknowledge the managed GPU preview")
	}
	switch spec.NodeSKU {
	case "Standard_NC4as_T4_v3":
		return AzureGPUCapacity{MilliCPU: 4000, MemoryMiB: 28 * 1024, GPUs: 1, MemoryPerGPUMiB: 16 * 1024}, nil
	case "Standard_NC8as_T4_v3":
		return AzureGPUCapacity{MilliCPU: 8000, MemoryMiB: 56 * 1024, GPUs: 1, MemoryPerGPUMiB: 16 * 1024}, nil
	default:
		return AzureGPUCapacity{}, fmt.Errorf("choose a reviewed NVIDIA T4 GPU node size")
	}
}
func azureGPUFit(fit *AzureClusterFit, plan Plan) error {
	gpu, err := AzureGPUPoolCapacity(fit.Spec.GPUPool)
	if err != nil {
		return err
	}
	copy := *fit.Spec.GPUPool
	fit.Spec.GPUPool = &copy
	systemCPU := fit.NominalNodeMilliCPU - fit.ReservedMilliCPUPerNode
	systemMemory := fit.NominalNodeMemoryMiB - fit.ReservedMemoryMiBPerNode
	gpuCPU, gpuMemory := gpu.MilliCPU-1000, gpu.MemoryMiB-4096
	cpuTotal, memoryTotal, gpuCPUTotal, gpuMemoryTotal, gpuTotal := 0, 0, 0, 0, 0
	block := func(path, code, message string) {
		fit.Fits = false
		fit.Decisions = append(fit.Decisions, Decision{path, code, message})
	}
	for _, s := range plan.Blueprint.Services {
		cpu, memory := systemCPU, systemMemory
		if s.Resources.GPUCount > 0 {
			cpu, memory = gpuCPU, gpuMemory
			gpuCPUTotal += s.Resources.RequestMilliCPU * s.Replicas
			gpuMemoryTotal += s.Resources.RequestMemoryMiB * s.Replicas
			gpuTotal += s.Resources.GPUCount * s.Replicas
			if s.Resources.GPUCount > gpu.GPUs {
				block("services."+s.ID, "gpu-pod-too-large", "One Pod requests more GPUs than a node in the selected pool.")
			}
		} else {
			cpuTotal += s.Resources.RequestMilliCPU * s.Replicas
			memoryTotal += s.Resources.RequestMemoryMiB * s.Replicas
		}
		if s.Resources.RequestMilliCPU > cpu || s.Resources.RequestMemoryMiB > memory {
			block("services."+s.ID, "pod-too-large", "One declared Pod exceeds its selected pool after explicit CPU and memory reserves.")
		}
	}
	if cpuTotal > systemCPU*fit.Spec.NodeCount || memoryTotal > systemMemory*fit.Spec.NodeCount {
		block("services.resources", "system-pool-too-small", "CPU workload requests exceed the fixed system pool after reserves.")
	}
	if gpuCPUTotal > gpuCPU*copy.NodeCount || gpuMemoryTotal > gpuMemory*copy.NodeCount || gpuTotal > gpu.GPUs*copy.NodeCount {
		block("services.resources", "gpu-pool-too-small", "GPU workload requests exceed the fixed GPU pool after reserves.")
	}
	fit.Decisions = append(fit.Decisions, Decision{"infrastructure.gpuPool", "managed-gpu-preview", "Explicit Azure managed GPU preview: fixed Ubuntu NVIDIA T4 nodes with provider-managed driver, device plugin and DCGM. The subscription feature must already be Registered. Each GPU has 16 GiB nominal device memory; model fit, quota, available capacity and price are unverified. No feature registration, autoscaling or SKU substitution is performed."})
	return nil
}
func azureGPUAgentPool(spec *AzureGPUNodePoolSpec) (map[string]any, error) {
	if _, err := AzureGPUPoolCapacity(spec); err != nil {
		return nil, err
	}
	return map[string]any{
		"name": "gpu", "count": spec.NodeCount, "vmSize": spec.NodeSKU, "mode": "User",
		"osType": "Linux", "osSKU": "Ubuntu", "type": "VirtualMachineScaleSets",
		"enableAutoScaling": false, "osDiskSizeGB": 128, "osDiskType": "Managed", "enableNodePublicIP": false,
		"nodeTaints": []string{"sku=gpu:NoSchedule"},
		"gpuProfile": map[string]any{"driver": "Install", "nvidia": map[string]string{"managementMode": "Managed"}},
	}, nil
}
