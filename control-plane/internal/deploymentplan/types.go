// Package deploymentplan describes inspectable deployment intent. It performs
// no I/O, discovers no credentials and implements no external Apply operation.
package deploymentplan

import (
	"context"
)

const Version = 1

type Blueprint struct {
	Version           int                `json:"version"`
	ProjectID         string             `json:"projectId"`
	Source            Source             `json:"source"`
	Infrastructure    Infrastructure     `json:"infrastructure"`
	Services          []Service          `json:"services"`
	ExternalResources []ExternalResource `json:"externalResources,omitempty"`
}

// Source is caller-supplied intent. Compilation does not attest to a commit,
// configuration revision, successful checks or permission to release it.
type Source struct {
	Commit                string `json:"commit"`
	ConfigurationRevision int    `json:"configurationRevision"`
}

type Infrastructure struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`                // kubernetes, existing-vm, or local-docker
	Reference string `json:"reference"`           // existing cluster/host identity; never credentials
	Namespace string `json:"namespace,omitempty"` // existing Kubernetes namespace
}

type ResourceBudget struct {
	RequestMilliCPU  int `json:"requestMilliCpu"`
	LimitMilliCPU    int `json:"limitMilliCpu"`
	RequestMemoryMiB int `json:"requestMemoryMiB"`
	LimitMemoryMiB   int `json:"limitMemoryMiB"`
	GPUCount         int `json:"gpuCount,omitempty"` // NVIDIA extended resource; no device installation
}

type ModelReference struct {
	Name         string `json:"name"`
	ClaimName    string `json:"claimName"` // existing PVC, provisioned and populated elsewhere
	MountPath    string `json:"mountPath"`
	RelativePath string `json:"relativePath,omitempty"`
}

type TelemetryBinding struct {
	ResourceID string `json:"resourceId"`
	Endpoint   string `json:"endpoint"` // existing OTLP/HTTP HTTPS receiver, without credentials
}

type Service struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`  // app or inference
	Image     string            `json:"image"` // explicit immutable OCI image@sha256 reference
	Port      int               `json:"port"`
	Replicas  int               `json:"replicas"`
	Resources ResourceBudget    `json:"resources"`
	Command   []string          `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Model     *ModelReference   `json:"model,omitempty"`
	Telemetry *TelemetryBinding `json:"telemetry,omitempty"`
	DependsOn []string          `json:"dependsOn,omitempty"`
}

// ExternalResource is an existing resource reference, not a request to create a
// database, analytics platform, secret, ingress or persistent volume.
type ExternalResource struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
}

type Decision struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Step struct {
	ComponentID string   `json:"componentId"`
	Action      string   `json:"action"`
	DependsOn   []string `json:"dependsOn"`
}

type Artifact struct {
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	SHA256    string `json:"sha256"`
	Content   string `json:"content"` // exact JSON manifest bytes, including final newline
}

type ResourceSummary struct {
	Replicas         int `json:"replicas"`
	RequestMilliCPU  int `json:"requestMilliCpu"`
	LimitMilliCPU    int `json:"limitMilliCpu"`
	RequestMemoryMiB int `json:"requestMemoryMiB"`
	LimitMemoryMiB   int `json:"limitMemoryMiB"`
	GPUCount         int `json:"gpuCount"`
}

type Capability struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	Description string `json:"description"`
}

type Plan struct {
	Version         int             `json:"version"`
	Digest          string          `json:"digest"`
	State           string          `json:"state"`
	Blueprint       Blueprint       `json:"blueprint"`
	Steps           []Step          `json:"steps"`
	Decisions       []Decision      `json:"decisions"`
	Artifacts       []Artifact      `json:"artifacts"`
	Capabilities    []Capability    `json:"capabilities"`
	Warnings        []string        `json:"warnings"`
	ResourceSummary ResourceSummary `json:"resourceSummary"`
	SourceVerified  bool            `json:"sourceVerified"`
	EffectsApplied  bool            `json:"effectsApplied"`
}

// Connector separates planning, fresh observation and authorized mutation.
// This package ships the contract, not a live connector implementation.
// Observe must retain ambiguous resource identities and report unknown rather
// than infer absence. Apply must not replay an unknown earlier operation.
type Connector interface {
	Plan(context.Context, Blueprint) (Plan, error)
	Observe(context.Context, ObservationRequest) (Observation, error)
	Apply(context.Context, ApplyRequest) (Observation, error)
}

type ObservationRequest struct {
	PlanDigest         string   `json:"planDigest"`
	OperationID        string   `json:"operationId"`
	ResourceReferences []string `json:"resourceReferences"`
}

type Observation struct {
	PlanDigest         string   `json:"planDigest"`
	OperationID        string   `json:"operationId"`
	State              string   `json:"state"` // unknown, absent, pending, ready, failed, or stopped
	ResourceReferences []string `json:"resourceReferences"`
	ObservedAt         string   `json:"observedAt"`
}

// ApprovalReference is an opaque reference to a separately verified authority
// record. Its presence is not authorization; a live adapter must reauthorize
// actor/project, exact plan, destination, source, effects and current costs.
type ApplyRequest struct {
	PlanDigest          string      `json:"planDigest"`
	OperationID         string      `json:"operationId"`
	ApprovalReference   string      `json:"approvalReference"`
	ExpectedObservation Observation `json:"expectedObservation"`
}
