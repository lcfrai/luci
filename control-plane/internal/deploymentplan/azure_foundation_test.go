package deploymentplan

import (
	"strings"
	"testing"
)

func foundationTestRequest(t *testing.T) AzureFoundationRequest {
	t.Helper()
	b := Blueprint{Version: 1, ProjectID: "p-one", Source: Source{Commit: strings.Repeat("a", 40)}, Infrastructure: Infrastructure{ID: "cluster", Kind: "kubernetes", Reference: "existing-cluster", Namespace: "apps"}, Services: []Service{{ID: "app", Kind: "app", Image: "example/app@sha256:" + strings.Repeat("b", 64), Port: 8080, Replicas: 2, Resources: ResourceBudget{RequestMilliCPU: 250, LimitMilliCPU: 500, RequestMemoryMiB: 256, LimitMemoryMiB: 512}}}}
	p, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	return AzureFoundationRequest{Blueprint: b, ExpectedBlueprintDigest: p.Digest}
}

func TestAzureFoundationUsesExactBlueprintAndExistingTierWithoutCapacityClaims(t *testing.T) {
	request := foundationTestRequest(t)
	fit, err := AzureFoundation(request, "small")
	if err != nil || !fit.Fits || fit.CapacityVerified || fit.MilliCPU != 2000 || fit.MemoryMiB != 4096 || fit.Requested.RequestMilliCPU != 500 || fit.Requested.RequestMemoryMiB != 512 || fit.SystemReserveMiB != 1024 || len(fit.Decisions) != 2 {
		t.Fatalf("fit: %+v %v", fit, err)
	}
	request.Blueprint.Services[0].Replicas = 3
	if _, err := AzureFoundation(request, "small"); err == nil {
		t.Fatal("changed blueprint reused the old reviewed digest")
	}
}

func TestAzureFoundationRejectsUnknownRequestsUnsupportedGPUAndOversizeTotals(t *testing.T) {
	for _, name := range []string{"unknown-requests", "gpu", "memory", "cpu"} {
		t.Run(name, func(t *testing.T) {
			r := foundationTestRequest(t)
			switch name {
			case "unknown-requests":
				r.Blueprint.Services[0].Replicas = 0
			case "gpu":
				r.Blueprint.Services[0].Resources.GPUCount = 1
			case "memory":
				r.Blueprint.Services[0].Resources.RequestMemoryMiB = 2048
				r.Blueprint.Services[0].Resources.LimitMemoryMiB = 2048
			case "cpu":
				r.Blueprint.Services[0].Resources.RequestMilliCPU = 1500
				r.Blueprint.Services[0].Resources.LimitMilliCPU = 1500
			}
			p, err := Compile(r.Blueprint)
			if err != nil {
				t.Fatal(err)
			}
			r.ExpectedBlueprintDigest = p.Digest
			fit, err := AzureFoundation(r, "small")
			if err != nil || fit.Fits || len(fit.Decisions) < 3 {
				t.Fatalf("unsupported request accepted: %+v %v", fit, err)
			}
		})
	}
}
