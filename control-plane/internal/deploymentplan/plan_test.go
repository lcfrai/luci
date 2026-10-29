package deploymentplan

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func exampleBlueprint() Blueprint {
	return Blueprint{Version: Version, ProjectID: "p-example", Source: Source{Commit: strings.Repeat("a", 40), ConfigurationRevision: 2},
		Infrastructure:    Infrastructure{ID: "cluster", Kind: "kubernetes", Reference: "existing-cluster-east", Namespace: "project-example"},
		ExternalResources: []ExternalResource{{ID: "telemetry", Kind: "analytics", Reference: "existing-otel-collector"}},
		Services: []Service{
			{ID: "web", Kind: "app", Image: "registry.example/web@sha256:" + strings.Repeat("b", 64), Port: 8080, Replicas: 2, Resources: ResourceBudget{100, 500, 128, 512, 0}, DependsOn: []string{"model"}},
			{ID: "model", Kind: "inference", Image: "registry.example/model-server@sha256:" + strings.Repeat("c", 64), Port: 8000, Replicas: 1, Resources: ResourceBudget{1000, 2000, 2048, 4096, 1}, Command: []string{"/server"}, Args: []string{"--model", "/models/selected"}, Model: &ModelReference{Name: "reviewed-model", ClaimName: "existing-models", MountPath: "/models", RelativePath: "approved"}, Telemetry: &TelemetryBinding{ResourceID: "telemetry", Endpoint: "https://telemetry.example/otlp"}},
		},
	}
}

func TestCompileAppAndInferencePreservesExplicitBindings(t *testing.T) {
	b := exampleBlueprint()
	p, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.State != "compiled-prototype" || len(p.Artifacts) != 4 || p.EffectsApplied || p.SourceVerified || len(p.Decisions) != 0 {
		t.Fatalf("unexpected plan: %+v", p)
	}
	deployments, services := 0, 0
	for _, artifact := range p.Artifacts {
		if artifact.SHA256 != digest([]byte(artifact.Content)) || artifact.MediaType != "application/json" {
			t.Fatal("artifact digest or media type mismatch")
		}
		if strings.HasSuffix(artifact.Name, ".deployment.json") {
			var d appsv1.Deployment
			if err := json.Unmarshal([]byte(artifact.Content), &d); err != nil {
				t.Fatal(err)
			}
			deployments++
			if d.APIVersion != "apps/v1" || d.Kind != "Deployment" || d.Namespace != "project-example" || !reflect.DeepEqual(d.Spec.Selector.MatchLabels, d.Spec.Template.Labels) {
				t.Fatalf("invalid deployment identity/selector: %+v", d)
			}
			pod := d.Spec.Template.Spec
			if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken || len(pod.Containers) != 1 {
				t.Fatal("unexpected pod permissions or containers")
			}
			c := pod.Containers[0]
			if c.Name == "model" {
				if c.Image != b.Services[1].Image || !reflect.DeepEqual(c.Command, b.Services[1].Command) || !reflect.DeepEqual(c.Args, b.Services[1].Args) {
					t.Fatal("compiler changed explicit inference image/entrypoint")
				}
				if len(pod.Volumes) != 1 || pod.Volumes[0].PersistentVolumeClaim.ClaimName != "existing-models" || !c.VolumeMounts[0].ReadOnly || c.VolumeMounts[0].SubPath != "approved" {
					t.Fatal("model was not bound read-only to the existing PVC")
				}
				gpu := c.Resources.Limits[corev1.ResourceName("nvidia.com/gpu")]
				if gpu.String() != "1" {
					t.Fatal("explicit GPU requirement was lost")
				}
				env := map[string]string{}
				for _, e := range c.Env {
					env[e.Name] = e.Value
				}
				if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "https://telemetry.example/otlp" || env["OTEL_EXPORTER_OTLP_PROTOCOL"] != "http/protobuf" {
					t.Fatal("telemetry binding changed")
				}
			}
		} else {
			var service corev1.Service
			if err := json.Unmarshal([]byte(artifact.Content), &service); err != nil {
				t.Fatal(err)
			}
			services++
			if service.APIVersion != "v1" || service.Kind != "Service" || service.Spec.Type != corev1.ServiceTypeClusterIP || len(service.Spec.Ports) != 1 || service.Spec.Ports[0].TargetPort.StrVal != "service" {
				t.Fatal("unexpected externally exposed or malformed Service")
			}
		}
	}
	if deployments != 2 || services != 2 {
		t.Fatal("missing actual resource manifests")
	}
	// The API must preserve the exact artifact bytes covered by each digest.
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Plan
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range roundTrip.Artifacts {
		if artifact.SHA256 != digest([]byte(artifact.Content)) {
			t.Fatal("JSON transport changed hashed manifest bytes")
		}
	}
	if p.ResourceSummary.Replicas != 3 || p.ResourceSummary.RequestMilliCPU != 1200 || p.ResourceSummary.GPUCount != 1 {
		t.Fatal("replica-weighted capacity summary is incorrect")
	}
	positions := map[string]int{}
	for i, step := range p.Steps {
		positions[step.ComponentID] = i
	}
	if positions["cluster"] >= positions["model"] || positions["telemetry"] >= positions["model"] || positions["model"] >= positions["web"] {
		t.Fatal("topology order lost a dependency")
	}
}

func TestCompileCanonicalizationIsStableAndProjectScoped(t *testing.T) {
	b := exampleBlueprint()
	before, _ := json.Marshal(b)
	first, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(b)
	if !bytes.Equal(before, after) {
		t.Fatal("compile mutated caller data")
	}
	b.Services[0], b.Services[1] = b.Services[1], b.Services[0]
	b.Services[1].DependsOn = []string{"model", "cluster", "model"}
	second, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || !reflect.DeepEqual(first.Artifacts, second.Artifacts) {
		t.Fatal("equivalent topology produced different artifacts")
	}
	b.ProjectID = "p-different"
	other, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	if other.Digest == first.Digest || other.Artifacts[0].Name == first.Artifacts[0].Name {
		t.Fatal("projects collide in a shared namespace")
	}
	b.Source.Commit = strings.Repeat("d", 40)
	changed, err := Compile(b)
	if err != nil || changed.Digest == other.Digest {
		t.Fatal("source revision not bound to plan")
	}
}

func TestCompileMissingInputsRemainDecisionsWithoutPartialManifests(t *testing.T) {
	b := exampleBlueprint()
	b.Source.Commit = ""
	b.Infrastructure.Reference = ""
	b.Services[1].Model = nil
	b.Services[0].Image = ""
	p, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.State != "decisions-required" || len(p.Decisions) != 4 || len(p.Artifacts) != 0 || len(p.Steps) != 4 {
		t.Fatalf("missing intent became a partial deployment: %+v", p)
	}
	b = exampleBlueprint()
	b.Infrastructure.Kind = "existing-vm"
	p, err = Compile(b)
	if err != nil || len(p.Artifacts) != 0 || len(p.Decisions) != 1 {
		t.Fatal("non-Kubernetes target was silently compiled")
	}
}

func TestCompileLongServiceNamesCannotOverwriteOtherArtifacts(t *testing.T) {
	b := exampleBlueprint()
	b.Services[0].ID = strings.Repeat("a", 49) + "-one"
	b.Services[0].DependsOn = nil
	b.Services[1] = b.Services[0]
	b.Services[1].ID = strings.Repeat("a", 49) + "-two"
	p, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, artifact := range p.Artifacts {
		if names[artifact.Name] {
			t.Fatal("long component IDs overwrite an artifact")
		}
		names[artifact.Name] = true
		var resource struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal([]byte(artifact.Content), &resource); err != nil || len(resource.Metadata.Name) > 63 {
			t.Fatal("invalid Kubernetes resource name")
		}
	}
	if len(names) != 4 {
		t.Fatal("missing distinct deployment/service artifacts")
	}
}

func TestCompileRejectsContradictoryOrUnsafeConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Blueprint)
	}{
		{"dependency cycle", func(b *Blueprint) { b.Services[1].DependsOn = []string{"web"} }},
		{"unknown dependency", func(b *Blueprint) { b.Services[0].DependsOn = []string{"absent"} }},
		{"duplicate identity", func(b *Blueprint) { b.Services[0].ID = "cluster" }},
		{"mutable image tag", func(b *Blueprint) { b.Services[0].Image = "registry.example/web:latest" }},
		{"invalid OCI repository", func(b *Blueprint) { b.Services[0].Image = "registry.example//web@sha256:" + strings.Repeat("a", 64) }},
		{"empty inference entrypoint", func(b *Blueprint) { b.Services[1].Command = []string{""} }},
		{"resource overcommit", func(b *Blueprint) { b.Services[0].Resources.RequestMilliCPU = 1000 }},
		{"relative mount", func(b *Blueprint) { b.Services[1].Model.MountPath = "models" }},
		{"model traversal", func(b *Blueprint) { b.Services[1].Model.RelativePath = "../private" }},
		{"credential-bearing endpoint", func(b *Blueprint) { b.Services[1].Telemetry.Endpoint = "https://user:secret@example.test" }},
		{"telemetry to non-analytics", func(b *Blueprint) { b.ExternalResources[0].Kind = "database" }},
		{"unknown component type", func(b *Blueprint) { b.Services[0].Kind = "managed-posthog" }},
		{"oversize topology", func(b *Blueprint) {
			for len(b.Services) < 33 {
				b.Services = append(b.Services, b.Services[0])
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := exampleBlueprint()
			tc.mutate(&b)
			if _, err := Compile(b); err == nil {
				t.Fatal("invalid blueprint accepted")
			}
		})
	}
}
