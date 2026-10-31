package deploymentplan

import (
	"encoding/json"
	appsv1 "k8s.io/api/apps/v1"
	"reflect"
	"strings"
	"testing"
)

func TestInferenceRuntimeCompilesExplicitProbesAndReferencesWithoutValues(t *testing.T) {
	b := exampleBlueprint()
	b.Services[1].DeploymentStrategy = "recreate"
	b.Services[1].Environment = []EnvironmentReference{{Name: "MODEL_CONFIG", Kind: "config-map", ResourceName: "model-options", Key: "config"}, {Name: "MODEL_TOKEN", Kind: "secret", ResourceName: "model-access", Key: "token"}}
	b.Services[1].Health = &ServiceHealth{Startup: &Probe{Kind: "tcp", PeriodSeconds: 10, TimeoutSeconds: 2, FailureThreshold: 60}, Readiness: &Probe{Kind: "http", Path: "/ready", PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 3}}
	before, _ := json.Marshal(b)
	p, err := Compile(b)
	if err != nil || len(p.Decisions) != 0 {
		t.Fatalf("compile: %+v %v", p.Decisions, err)
	}
	after, _ := json.Marshal(b)
	if string(before) != string(after) {
		t.Fatal("compiler mutated caller topology")
	}
	found := false
	for _, artifact := range p.Artifacts {
		if !strings.HasSuffix(artifact.Name, ".deployment.json") {
			continue
		}
		var d appsv1.Deployment
		if json.Unmarshal([]byte(artifact.Content), &d) != nil {
			t.Fatal("invalid manifest")
		}
		c := d.Spec.Template.Spec.Containers[0]
		if c.Name != "model" {
			continue
		}
		found = true
		if d.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType || c.StartupProbe == nil || c.StartupProbe.TCPSocket == nil || c.StartupProbe.TCPSocket.Port.IntValue() != 8000 || c.ReadinessProbe == nil || c.ReadinessProbe.HTTPGet.Path != "/ready" || c.ReadinessProbe.HTTPGet.Port.IntValue() != 8000 || len(c.ReadinessProbe.HTTPGet.HTTPHeaders) != 0 || c.ReadinessProbe.HTTPGet.Host != "" {
			t.Fatal("explicit same-port probe or strategy lost")
		}
		env := c.Env
		if len(env) != 5 || env[0].Name != "MODEL_CONFIG" || env[0].Value != "" || env[0].ValueFrom.ConfigMapKeyRef.Name != "model-options" || env[1].ValueFrom.SecretKeyRef.Key != "token" || *env[1].ValueFrom.SecretKeyRef.Optional {
			t.Fatal("reference or existing telemetry binding lost")
		}
	}
	if !found {
		t.Fatal("model deployment absent")
	}
}

func TestInferenceProbeRejectsCredentialChannelsAndContradictions(t *testing.T) {
	for _, path := range []string{"https://other/ready", "//other/ready", "/ready?token=secret", "/ready#token", "/%0aheader", "/%3fquery", "/\\other"} {
		t.Run(path, func(t *testing.T) {
			b := exampleBlueprint()
			b.Services[0].Health = &ServiceHealth{Readiness: &Probe{Kind: "http", Path: path, PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 3}}
			if _, err := Compile(b); err == nil {
				t.Fatal("invalid probe accepted")
			}
		})
	}
	b := exampleBlueprint()
	b.Services[0].Health = &ServiceHealth{Startup: &Probe{Kind: "tcp", Path: "/not-tcp", PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 3}}
	if _, err := Compile(b); err == nil {
		t.Fatal("TCP path accepted")
	}
	b = exampleBlueprint()
	b.Services[0].Environment = []EnvironmentReference{{Name: "TOKEN", Kind: "secret", ResourceName: "access", Key: "one"}, {Name: "TOKEN", Kind: "secret", ResourceName: "access", Key: "two"}}
	if _, err := Compile(b); err == nil {
		t.Fatal("duplicate environment accepted")
	}
	b = exampleBlueprint()
	b.Services[0].Health = &ServiceHealth{Readiness: &Probe{Kind: "http", PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 3}}
	p, err := Compile(b)
	if err != nil || len(p.Decisions) == 0 || len(p.Artifacts) != 0 {
		t.Fatal("unknown image path became an invented probe")
	}
}

func TestInferenceAbsentOptionalRuntimeFieldsPreserveLegacyBytes(t *testing.T) {
	b := exampleBlueprint()
	legacy, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	b.Services[0].Environment = []EnvironmentReference{}
	same, err := Compile(b)
	if err != nil || legacy.Digest != same.Digest || !reflect.DeepEqual(legacy.Artifacts, same.Artifacts) {
		t.Fatal("absent/empty optional refs changed legacy artifacts")
	}
}
