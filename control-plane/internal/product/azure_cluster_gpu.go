package product

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

type azureManagedClusterPool struct {
	Name       string   `json:"name"`
	VMSize     string   `json:"vmSize"`
	Count      int      `json:"count"`
	Mode       string   `json:"mode"`
	Auto       bool     `json:"enableAutoScaling"`
	OSType     string   `json:"osType"`
	OSSKU      string   `json:"osSKU"`
	PublicIP   bool     `json:"enableNodePublicIP"`
	NodeTaints []string `json:"nodeTaints"`
	State      string   `json:"provisioningState"`
	GPUProfile *struct {
		Driver string `json:"driver"`
		Nvidia *struct {
			ManagementMode string `json:"managementMode"`
		} `json:"nvidia"`
	} `json:"gpuProfile"`
}

func azureClusterPoolsMatch(pools []azureManagedClusterPool, spec deploymentplan.AzureClusterSpec) bool {
	count := 1
	if spec.GPUPool != nil {
		count++
	}
	if len(pools) != count {
		return false
	}
	foundSystem, foundGPU := false, false
	for _, pool := range pools {
		if pool.Auto || pool.OSType != "Linux" || pool.OSSKU != "Ubuntu" || pool.PublicIP {
			return false
		}
		switch pool.Name {
		case "system":
			if foundSystem || pool.VMSize != spec.NodeSKU || pool.Count != spec.NodeCount || pool.Mode != "System" {
				return false
			}
			foundSystem = true
		case "gpu":
			if foundGPU || spec.GPUPool == nil {
				return false
			}
			if _, err := deploymentplan.AzureGPUPoolCapacity(spec.GPUPool); err != nil {
				return false
			}
			if pool.VMSize != spec.GPUPool.NodeSKU || pool.Count != spec.GPUPool.NodeCount || pool.Mode != "User" || len(pool.NodeTaints) != 1 || pool.NodeTaints[0] != "sku=gpu:NoSchedule" || pool.GPUProfile == nil || pool.GPUProfile.Driver != "Install" || pool.GPUProfile.Nvidia == nil || pool.GPUProfile.Nvidia.ManagementMode != "Managed" {
				return false
			}
			foundGPU = true
		default:
			return false
		}
	}
	return foundSystem && (spec.GPUPool == nil || foundGPU)
}
func managedGPUFeatureRegistered(raw string, spec deploymentplan.AzureClusterSpec) bool {
	var value struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Properties struct {
			State string `json:"state"`
		} `json:"properties"`
	}
	expected := "/subscriptions/" + spec.SubscriptionID + "/providers/Microsoft.Features/providers/Microsoft.ContainerService/features/" + deploymentplan.AzureManagedGPUFeature
	return decodeBoundedAzureJSON(raw, &value) == nil && strings.EqualFold(value.ID, expected) && value.Name == "Microsoft.ContainerService/"+deploymentplan.AzureManagedGPUFeature && value.Properties.State == "Registered"
}

// Read only. Registration, extension installation and billing changes are never
// implicit in cluster review or creation. Old CPU plans bypass this entirely.
func (s *Server) checkManagedGPUFeature(ctx context.Context, actor management.ExecutionActorRef, spec deploymentplan.AzureClusterSpec) error {
	if spec.GPUPool == nil {
		return nil
	}
	if _, err := deploymentplan.AzureGPUPoolCapacity(spec.GPUPool); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return management.ErrExecutionAuthority
	}
	raw, err := s.runner(ctx, "az", []string{"feature", "show", "--namespace", "Microsoft.ContainerService", "--name", deploymentplan.AzureManagedGPUFeature, "--subscription", spec.SubscriptionID, "--output", "json"}, "")
	if err != nil || len(raw) > 64<<10 || ctx.Err() != nil || !managedGPUFeatureRegistered(raw, spec) {
		return fmt.Errorf("The selected subscription must already have ManagedGPUExperiencePreview Registered. Confirm that preview enrollment in Azure, or choose a CPU cluster. Luci has not registered the feature. Inspect the original cluster operation before continuing.")
	}
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return management.ErrExecutionAuthority
	}
	return nil
}
