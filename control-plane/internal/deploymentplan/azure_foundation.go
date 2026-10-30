package deploymentplan

import "fmt"

// AzureFoundationRequest selects the existing single-VM Azure setup adapter.
// A matching compiler digest binds intent; it grants no project/provider access.
type AzureFoundationRequest struct {
	Blueprint               Blueprint `json:"blueprint"`
	ExpectedBlueprintDigest string    `json:"expectedBlueprintDigest"`
}

type AzureFoundationFit struct {
	BlueprintDigest  string          `json:"blueprintDigest"`
	Tier             string          `json:"tier"`
	MilliCPU         int             `json:"milliCpu"`
	MemoryMiB        int             `json:"memoryMiB"`
	SystemReserveMiB int             `json:"systemReserveMiB"`
	Requested        ResourceSummary `json:"requested"`
	Fits             bool            `json:"fits"`
	CapacityVerified bool            `json:"capacityVerified"`
	Decisions        []Decision      `json:"decisions"`
}

// AzureFoundation checks declared requests against the exact existing B-series
// setup tiers. It does not quote, discover capacity, compile a VM workload or
// claim Kubernetes/PVC/model/GPU installation. Those remain separate decisions.
func AzureFoundation(request AzureFoundationRequest, tier string) (AzureFoundationFit, error) {
	plan, err := Compile(request.Blueprint)
	if err != nil {
		return AzureFoundationFit{}, err
	}
	if request.ExpectedBlueprintDigest != plan.Digest {
		return AzureFoundationFit{}, fmt.Errorf("blueprint changed; regenerate the reviewed plan")
	}
	fit := AzureFoundationFit{BlueprintDigest: plan.Digest, Tier: tier, SystemReserveMiB: 1024, Requested: plan.ResourceSummary, Fits: true, Decisions: []Decision{}}
	switch tier {
	case "small":
		fit.MilliCPU, fit.MemoryMiB = 2000, 4096
	case "standard":
		fit.MilliCPU, fit.MemoryMiB = 2000, 8192
	case "full":
		fit.MilliCPU, fit.MemoryMiB = 4000, 8192
	default:
		return fit, fmt.Errorf("choose an existing Azure small, standard or full foundation tier")
	}
	block := func(path, code, message string) {
		fit.Fits = false
		fit.Decisions = append(fit.Decisions, Decision{path, code, message})
	}
	for _, service := range plan.Blueprint.Services {
		if service.Replicas < 1 || service.Resources.RequestMilliCPU < 1 || service.Resources.RequestMemoryMiB < 1 {
			block("services."+service.ID, "requests-required", "Choose replica, CPU and memory requests before sizing a foundation.")
		}
	}
	if fit.Requested.RequestMilliCPU > fit.MilliCPU || fit.Requested.RequestMemoryMiB > fit.MemoryMiB-fit.SystemReserveMiB {
		block("services.resources", "tier-too-small", "Declared aggregate requests exceed this tier after the explicit 1 GiB system reserve. Choose another tier or revise the workload.")
	}
	if fit.Requested.GPUCount > 0 {
		block("services.resources.gpuCount", "gpu-unavailable", "These existing Azure B-series foundation tiers have no GPU. Do not use this foundation as an inference capacity claim.")
	}
	fit.Decisions = append(fit.Decisions, Decision{"infrastructure", "capacity-unverified", "The selected SKU is nominal sizing, not observed quota, availability, free memory or a verified price. Provider validation and actual workload admission remain separate."})
	fit.Decisions = append(fit.Decisions, Decision{"services", "workloads-unapplied", "This operation creates only the existing Docker/Coolify VM foundation. Kubernetes, application images, model storage, inference and telemetry are not applied or observed by it."})
	return fit, nil
}
