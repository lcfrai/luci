package deploymentplan

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestManagedGPUClusterCompilesExplicitPoolWithoutChangingCPUDefault(t *testing.T) {
	request, spec, principal := clusterPlanFixture(t)
	cpuFit, err := AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab")
	if err != nil || !cpuFit.Fits || AzureClusterAPI(spec) != AzureManagedClusterAPIVersion {
		t.Fatal("legacy CPU path changed", err)
	}
	encoded, _ := json.Marshal(spec)
	if strings.Contains(string(encoded), "gpuPool") {
		t.Fatal("optional GPU field changed legacy JSON")
	}
	request.Blueprint.Services[0].Resources.GPUCount = 1
	compiled, err := Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedBlueprintDigest = compiled.Digest
	missing, err := AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab")
	if err != nil || missing.Fits {
		t.Fatal("GPU workload admitted without a selected pool", err)
	}
	spec.GPUPool = &AzureGPUNodePoolSpec{NodeSKU: "Standard_NC4as_T4_v3", NodeCount: 1, AcknowledgeManagedPreview: true}
	fit, err := AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab")
	if err != nil || !fit.Fits || fit.CapacityVerified {
		t.Fatal("bounded GPU plan unavailable", err)
	}
	raw, err := AzureClusterTemplate(fit, "project-east", "eastus", "ssh-ed25519 "+base64.StdEncoding.EncodeToString(make([]byte, 32)), "cp-original")
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Resources []struct {
			APIVersion string `json:"apiVersion"`
			Properties struct {
				Pools []struct {
					Name  string `json:"name"`
					Count int    `json:"count"`
					Mode  string `json:"mode"`
					Auto  bool   `json:"enableAutoScaling"`
					GPU   struct {
						Driver string `json:"driver"`
						Nvidia struct {
							Mode string `json:"managementMode"`
						} `json:"nvidia"`
					} `json:"gpuProfile"`
				} `json:"agentPoolProfiles"`
			} `json:"properties"`
		}
	}
	if json.Unmarshal([]byte(raw), &template) != nil || len(template.Resources) != 1 {
		t.Fatal("invalid template")
	}
	resource := template.Resources[0]
	if resource.APIVersion != AzureManagedGPUAPIVersion || len(resource.Properties.Pools) != 2 {
		t.Fatal("wrong API or hidden pool")
	}
	gpu := resource.Properties.Pools[1]
	if gpu.Name != "gpu" || gpu.Count != 1 || gpu.Mode != "User" || gpu.Auto || gpu.GPU.Driver != "Install" || gpu.GPU.Nvidia.Mode != "Managed" {
		t.Fatal("managed driver/plugin stack not exact")
	}
	if strings.Contains(raw, "Microsoft.Features") || strings.Contains(raw, "roleAssignments") {
		t.Fatal("hidden provider feature or access mutation")
	}
	spec.GPUPool.NodeCount = 4
	if fit.Spec.GPUPool.NodeCount != 1 {
		t.Fatal("fit aliased mutable request")
	}
}
func TestManagedGPUFitSeparatesPoolCapacityAndRejectsUnreviewedExpansion(t *testing.T) {
	request, spec, principal := clusterPlanFixture(t)
	request.Blueprint.Services[0].Resources.GPUCount = 2
	compiled, err := Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedBlueprintDigest = compiled.Digest
	spec.GPUPool = &AzureGPUNodePoolSpec{NodeSKU: "Standard_NC4as_T4_v3", NodeCount: 4, AcknowledgeManagedPreview: true}
	fit, err := AzureClusterFoundation(request, spec, principal, "project-east", "0123456789ab")
	if err != nil || fit.Fits {
		t.Fatal("multi-GPU Pod fitted across different nodes", err)
	}
	for _, choice := range []AzureGPUNodePoolSpec{{NodeSKU: "Standard_NC4as_T4_v3", NodeCount: 1}, {NodeSKU: "Standard_NC4as_T4_v3", NodeCount: 5, AcknowledgeManagedPreview: true}, {NodeSKU: "unreviewed", NodeCount: 1, AcknowledgeManagedPreview: true}} {
		if _, err := AzureGPUPoolCapacity(&choice); err == nil {
			t.Fatal("unreviewed GPU choice accepted")
		}
	}
}
func TestGPUWorkloadToleratesOnlyTheExplicitGPUNodeTaint(t *testing.T) {
	request, _, _ := clusterPlanFixture(t)
	request.Blueprint.Services[0].Resources.GPUCount = 1
	compiled, err := Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, artifact := range compiled.Artifacts {
		if !strings.HasSuffix(artifact.Name, ".deployment.json") {
			continue
		}
		var value struct {
			Spec struct {
				Template struct {
					Spec struct {
						Tolerations []struct{ Key, Operator, Value, Effect string }
					}
				}
			}
		}
		if json.Unmarshal([]byte(artifact.Content), &value) != nil {
			t.Fatal("invalid deployment")
		}
		ts := value.Spec.Template.Spec.Tolerations
		if len(ts) != 1 || ts[0].Key != "sku" || ts[0].Operator != "Equal" || ts[0].Value != "gpu" || ts[0].Effect != "NoSchedule" {
			t.Fatal("GPU task cannot schedule, or toleration is broad")
		}
		found = true
	}
	if !found {
		t.Fatal("GPU deployment missing")
	}
}
