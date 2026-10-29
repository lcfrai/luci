package deploymentplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/distribution/reference"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var imagePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]*@sha256:[0-9a-f]{64}$`)

func validName(value string) bool { return len(value) <= 63 && namePattern.MatchString(value) }
func text(value string, limit int) bool {
	if len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	for _, c := range value {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

// Compile returns a deterministic, no-I/O plan. Unknown required inputs become
// decisions. Invalid or contradictory topology returns an error. Artifacts are
// emitted only when every required input is resolved, never for a partial DAG.
func Compile(input Blueprint) (Plan, error) {
	// Deep copy so sorting, canonicalization and returned values cannot mutate
	// the caller's service/dependency slices.
	raw, err := json.Marshal(input)
	if err != nil || len(raw) > 128<<10 {
		return Plan{}, fmt.Errorf("blueprint exceeds 128 KiB")
	}
	var b Blueprint
	if err := json.Unmarshal(raw, &b); err != nil {
		return Plan{}, err
	}
	if b.Version != Version || !text(b.ProjectID, 128) || b.ProjectID == "" || len(b.Services) == 0 || len(b.Services) > 32 || len(b.ExternalResources) > 64 {
		return Plan{}, fmt.Errorf("choose version 1, a project and 1–32 services with at most 64 external resources")
	}
	if b.Source.ConfigurationRevision < 0 || b.Source.ConfigurationRevision > 1_000_000 {
		return Plan{}, fmt.Errorf("invalid configuration revision")
	}
	if b.Source.Commit != "" && !commitPattern.MatchString(b.Source.Commit) {
		return Plan{}, fmt.Errorf("source commit must be a full lowercase Git SHA-1")
	}
	if !validName(b.Infrastructure.ID) || !text(b.Infrastructure.Reference, 512) {
		return Plan{}, fmt.Errorf("invalid infrastructure identity")
	}
	if b.Infrastructure.Kind != "kubernetes" && b.Infrastructure.Kind != "existing-vm" && b.Infrastructure.Kind != "local-docker" {
		return Plan{}, fmt.Errorf("unknown infrastructure kind")
	}
	if b.Infrastructure.Namespace != "" && !validName(b.Infrastructure.Namespace) {
		return Plan{}, fmt.Errorf("invalid namespace")
	}
	p := Plan{Version: Version, State: "decisions-required", Decisions: []Decision{}, Steps: []Step{}, Artifacts: []Artifact{}, Capabilities: capabilities(), Warnings: []string{
		"Prototype manifests only: no cluster, namespace, image, model, PVC, device plugin or telemetry receiver is created or verified.",
		"Source and configuration references are caller supplied; successful checks, current access and release authority are not verified by compilation.",
		"No quote or billing cap is established. Review provider capacity, image licences, security, health probes, exposure, costs and recovery before any separate apply.",
	}}
	decision := func(field, code, message string) { p.Decisions = append(p.Decisions, Decision{field, code, message}) }
	if b.Source.Commit == "" {
		decision("source.commit", "source-required", "Choose the exact application source revision.")
	}
	if b.Infrastructure.Reference == "" {
		decision("infrastructure.reference", "existing-target-required", "Identify the existing target; provisioning is not implemented by this compiler.")
	}
	if b.Infrastructure.Kind != "kubernetes" {
		decision("infrastructure.kind", "compiler-unavailable", "This prototype emits Kubernetes manifests only; existing Local/VM release paths remain separate.")
	}
	if b.Infrastructure.Kind == "kubernetes" && b.Infrastructure.Namespace == "" {
		decision("infrastructure.namespace", "namespace-required", "Choose an existing Kubernetes namespace.")
	}
	sort.Slice(b.Services, func(i, j int) bool { return b.Services[i].ID < b.Services[j].ID })
	sort.Slice(b.ExternalResources, func(i, j int) bool { return b.ExternalResources[i].ID < b.ExternalResources[j].ID })
	nodes := map[string][]string{b.Infrastructure.ID: {}}
	externals := map[string]ExternalResource{}
	for i, resource := range b.ExternalResources {
		if !validName(resource.ID) || !text(resource.Reference, 512) {
			return Plan{}, fmt.Errorf("invalid external resource")
		}
		if _, exists := nodes[resource.ID]; exists {
			return Plan{}, fmt.Errorf("duplicate component %q", resource.ID)
		}
		switch resource.Kind {
		case "database", "analytics", "storage", "secret", "ingress":
		default:
			return Plan{}, fmt.Errorf("unknown external resource kind")
		}
		nodes[resource.ID] = []string{b.Infrastructure.ID}
		externals[resource.ID] = resource
		if resource.Reference == "" {
			decision(fmt.Sprintf("externalResources[%d].reference", i), "external-reference-required", "Identify the existing resource; this is not a provisioning request.")
		}
	}
	for i := range b.Services {
		s := &b.Services[i]
		field := fmt.Sprintf("services[%d]", i)
		if !validName(s.ID) || (s.Kind != "app" && s.Kind != "inference") {
			return Plan{}, fmt.Errorf("invalid service identity or kind")
		}
		if _, exists := nodes[s.ID]; exists {
			return Plan{}, fmt.Errorf("duplicate component %q", s.ID)
		}
		if s.Image == "" {
			decision(field+".image", "image-required", "Choose an existing immutable OCI image digest.")
		} else if !validImage(s.Image) {
			return Plan{}, fmt.Errorf("%s image must use an explicit sha256 digest", s.ID)
		}
		if s.Port == 0 {
			decision(field+".port", "port-required", "Specify the container and internal Service port.")
		} else if s.Port < 1 || s.Port > 65535 {
			return Plan{}, fmt.Errorf("invalid service port")
		}
		if s.Replicas == 0 {
			decision(field+".replicas", "replicas-required", "Choose 1–32 replicas for this prototype.")
		} else if s.Replicas < 1 || s.Replicas > 32 {
			return Plan{}, fmt.Errorf("invalid replicas")
		}
		r := s.Resources
		if r.RequestMilliCPU < 0 || r.LimitMilliCPU < 0 || r.RequestMemoryMiB < 0 || r.LimitMemoryMiB < 0 || r.GPUCount < 0 || r.GPUCount > 8 || r.LimitMilliCPU > 64000 || r.LimitMemoryMiB > 1048576 {
			return Plan{}, fmt.Errorf("invalid resource budget")
		}
		if r.RequestMilliCPU == 0 || r.LimitMilliCPU == 0 || r.RequestMemoryMiB == 0 || r.LimitMemoryMiB == 0 {
			decision(field+".resources", "resources-required", "Specify nonzero CPU and memory requests and limits.")
		}
		if r.RequestMilliCPU > r.LimitMilliCPU || r.RequestMemoryMiB > r.LimitMemoryMiB {
			return Plan{}, fmt.Errorf("resource requests exceed limits")
		}
		if len(s.Command) > 16 || len(s.Args) > 64 {
			return Plan{}, fmt.Errorf("command or argument limit exceeded")
		}
		for _, arg := range append(append([]string{}, s.Command...), s.Args...) {
			if !text(arg, 1024) {
				return Plan{}, fmt.Errorf("invalid command or argument")
			}
		}
		for _, arg := range s.Command {
			if strings.TrimSpace(arg) == "" {
				return Plan{}, fmt.Errorf("command elements must not be empty")
			}
		}
		if s.Kind == "app" && s.Model != nil {
			return Plan{}, fmt.Errorf("only inference services accept model mounts")
		}
		if s.Kind == "inference" {
			if s.Model == nil {
				decision(field+".model", "model-required", "Choose an existing model PVC and explicit container mount; no model is downloaded.")
			} else {
				m := s.Model
				if m.Name == "" || m.ClaimName == "" || m.MountPath == "" {
					decision(field+".model", "model-reference-required", "Specify a model name, existing PVC and mount path.")
				}
				if !text(m.Name, 128) || (m.ClaimName != "" && !validName(m.ClaimName)) || !validMount(m.MountPath) || !validRelative(m.RelativePath) {
					return Plan{}, fmt.Errorf("invalid model mount")
				}
			}
			if len(s.Command) == 0 && len(s.Args) == 0 {
				decision(field+".command", "server-configuration-required", "Provide the image-specific inference entrypoint or arguments; the compiler does not invent a model-server CLI.")
			}
		}
		deps := append([]string{b.Infrastructure.ID}, s.DependsOn...)
		if len(deps) > 65 {
			return Plan{}, fmt.Errorf("too many dependencies")
		}
		if s.Telemetry != nil {
			t := s.Telemetry
			resource, exists := externals[t.ResourceID]
			if !exists || resource.Kind != "analytics" {
				return Plan{}, fmt.Errorf("telemetry requires an analytics resource dependency")
			}
			if t.Endpoint == "" {
				decision(field+".telemetry.endpoint", "telemetry-endpoint-required", "Supply an existing OTLP/HTTP receiver endpoint.")
			} else if !validEndpoint(t.Endpoint) {
				return Plan{}, fmt.Errorf("telemetry endpoint must be HTTPS without credentials, query or fragment")
			}
			deps = append(deps, t.ResourceID)
		}
		s.DependsOn = uniqueSorted(deps)
		nodes[s.ID] = s.DependsOn
		p.ResourceSummary.Replicas += s.Replicas
		p.ResourceSummary.RequestMilliCPU += r.RequestMilliCPU * s.Replicas
		p.ResourceSummary.LimitMilliCPU += r.LimitMilliCPU * s.Replicas
		p.ResourceSummary.RequestMemoryMiB += r.RequestMemoryMiB * s.Replicas
		p.ResourceSummary.LimitMemoryMiB += r.LimitMemoryMiB * s.Replicas
		p.ResourceSummary.GPUCount += r.GPUCount * s.Replicas
	}
	order, err := dependencyOrder(nodes)
	if err != nil {
		return Plan{}, err
	}
	for _, id := range order {
		action := "review-existing-resource"
		if id == b.Infrastructure.ID {
			action = "review-existing-infrastructure"
		} else if _, exists := externals[id]; !exists {
			action = "compile-deployment-and-internal-service"
		}
		p.Steps = append(p.Steps, Step{id, action, uniqueSorted(nodes[id])})
	}
	p.Blueprint = b
	canonical, err := json.Marshal(b)
	if err != nil {
		return Plan{}, err
	}
	p.Digest = digest(canonical)
	if len(p.Decisions) == 0 {
		p.State = "compiled-prototype"
		p.Artifacts, err = manifests(b, p.Digest)
		if err != nil {
			return Plan{}, err
		}
	}
	return p, nil
}

func validMount(value string) bool {
	return value == "" || (text(value, 256) && strings.HasPrefix(value, "/") && value != "/" && path.Clean(value) == value && !strings.Contains(value, "\\"))
}
func validRelative(value string) bool {
	return value == "" || (text(value, 256) && !strings.HasPrefix(value, "/") && !strings.Contains(value, "\\") && path.Clean(value) == value && value != "." && value != ".." && !strings.HasPrefix(value, "../"))
}
func validEndpoint(value string) bool {
	u, err := url.Parse(value)
	return text(value, 2048) && err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
func validImage(value string) bool {
	if len(value) > 512 || !imagePattern.MatchString(value) {
		return false
	}
	_, err := reference.ParseNormalizedNamed(value)
	return err == nil
}
func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func uniqueSorted(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	for i := 1; i < len(out); {
		if out[i] == out[i-1] {
			out = append(out[:i], out[i+1:]...)
		} else {
			i++
		}
	}
	return out
}
func dependencyOrder(nodes map[string][]string) ([]string, error) {
	keys := make([]string, 0, len(nodes))
	for id := range nodes {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	states := map[string]int{}
	out := []string{}
	var visit func(string) error
	visit = func(id string) error {
		deps, exists := nodes[id]
		if !exists {
			return fmt.Errorf("unknown dependency %q", id)
		}
		if states[id] == 1 {
			return fmt.Errorf("dependency cycle at %q", id)
		}
		if states[id] == 2 {
			return nil
		}
		states[id] = 1
		for _, dep := range uniqueSorted(deps) {
			if err := visit(dep); err != nil {
				return err
			}
		}
		states[id] = 2
		out = append(out, id)
		return nil
	}
	for _, id := range keys {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func capabilities() []Capability {
	return []Capability{
		{"kubernetes-manifests", "prototype", "Compile app/inference Deployment and ClusterIP Service JSON for an existing namespace; no Apply implementation."},
		{"telemetry-binding", "prototype", "Bind an existing OTLP/HTTP receiver via environment variables; no instrumentation or analytics platform provisioning."},
		{"local-pages-coolify", "existing-separate-path", "Existing project release executors retain their own admission, observation and acceptance gates; this compiler does not invoke them."},
		{"aws-azure-vm", "existing-separate-path", "Existing one-machine provisioning is separate from managed Kubernetes, customer inference and cloud microVM acceptance."},
		{"cluster-model-analytics-provisioning", "planned", "Cluster creation, model acquisition, GPU setup, analytics provisioning and live connector commissioning remain unimplemented here."},
	}
}
