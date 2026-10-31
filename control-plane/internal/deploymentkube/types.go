// Package deploymentkube executes the compiler's owned Deployment, ClusterIP
// Service and optional reviewed HTTPS Ingress in an existing namespace.
// It does not provision infrastructure, acquire images/models or load credentials.
package deploymentkube

import (
	"context"
	"io"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"k8s.io/client-go/kubernetes"
)

type Identity struct {
	ProjectID         string `json:"projectId"`
	ActorID           string `json:"actorId"`
	TeamID            string `json:"teamId,omitempty"`
	TargetID          string `json:"targetId"`
	TargetFingerprint string `json:"targetFingerprint"`
	OperationID       string `json:"operationId"`
	PlanDigest        string `json:"planDigest"`
}

type ExpectedResource struct {
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	UID             string `json:"uid"` // empty UID AND version means reviewed absence
	ResourceVersion string `json:"resourceVersion"`
}

type Request struct {
	PublicRouteBinding *PublicRouteBinding      `json:"publicRouteBinding,omitempty"`
	Identity           Identity                 `json:"identity"`
	Blueprint          deploymentplan.Blueprint `json:"blueprint"`
	ApprovalReference  string                   `json:"approvalReference"`
	Expected           []ExpectedResource       `json:"expected"`
	References         []ReferenceSnapshot      `json:"references,omitempty"`
}

// Only selected keys and bounded identity/access metadata enter a review.
// Secret/ConfigMap values are neither hashed nor retained in this record.
type ReferenceSnapshot struct {
	CapacityBytes     string   `json:"capacityBytes,omitempty"`
	Kind              string   `json:"kind"`
	Name              string   `json:"name"`
	Namespace         string   `json:"namespace"`
	NamespaceUID      string   `json:"namespaceUid"`
	TargetFingerprint string   `json:"targetFingerprint"`
	UID               string   `json:"uid"`
	ResourceVersion   string   `json:"resourceVersion"`
	Keys              []string `json:"keys,omitempty"`
	Phase             string   `json:"phase,omitempty"`
	AccessModes       []string `json:"accessModes,omitempty"`
	VolumeMode        string   `json:"volumeMode,omitempty"`
	VolumeName        string   `json:"volumeName,omitempty"`
	MetadataDigest    string   `json:"metadataDigest"`
}

type Target struct {
	ID           string
	Fingerprint  string
	Namespace    string
	NamespaceUID string
	Client       kubernetes.Interface
}

// Resolver must select an already approved private target and TLS/authenticated
// client. Browser-provided kubeconfig, URLs and credentials never enter this API.
type Resolver func(context.Context, Identity) (Target, error)

// Authorizer rechecks the ORIGINAL actor/project and exact plan/target/source/
// checks/approval. mutation additionally requires current acknowledged effects
// and cost/capacity policy. It must not treat ApprovalReference as authority.
// Observation still requires current access; it never demands new allocation.
type Authorizer func(context.Context, Request, bool) error

type ResourceReceipt struct {
	ExpectedResource
	DesiredSHA256      string `json:"desiredSha256"`
	Attempted          bool   `json:"attempted"`
	ObservedUID        string `json:"observedUid,omitempty"`
	ObservedVersion    string `json:"observedVersion,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	Ready              bool   `json:"ready"`
}

type Operation struct {
	PublicRoute               *PublicRouteObservation `json:"publicRoute,omitempty"`
	Revision                  int                     `json:"revision"`
	Request                   Request                 `json:"request"`
	RequestDigest             string                  `json:"requestDigest"`
	NamespaceUID              string                  `json:"namespaceUid"`
	State                     string                  `json:"state"` // reserved, unknown, pending, ready, blocked
	Resources                 []ResourceReceipt       `json:"resources"`
	ObservedAt                string                  `json:"observedAt"`
	ApplicationHealthVerified bool                    `json:"applicationHealthVerified"`
	ReferenceState            string                  `json:"referenceState,omitempty"` // current, changed, unavailable, unreviewed
	Workloads                 []WorkloadObservation   `json:"workloads,omitempty"`
	Telemetry                 *TelemetryObservation   `json:"telemetry,omitempty"` // latest retained attributed sample, not live/lifetime analytics
}

type WorkloadObservation struct {
	ComponentID              string   `json:"componentId"`
	DeploymentUID            string   `json:"deploymentUid"`
	State                    string   `json:"state"` // ready, pending, unknown, unprobed
	ReadinessProbeConfigured bool     `json:"readinessProbeConfigured"`
	ObservedPods             int      `json:"observedPods"`
	ReadyPods                int      `json:"readyPods"`
	RestartCount             int64    `json:"restartCount"`
	OOMObserved              bool     `json:"oomObserved"`
	Reasons                  []string `json:"reasons"` // closed codes only, never event/log/error text
	Omitted                  bool     `json:"omitted"`
	ObservedAt               string   `json:"observedAt"`
	ModelContentsVerified    bool     `json:"modelContentsVerified"`
}

// Journal reservations and CAS writes MUST be durable before they return. A
// failed write never permits another provider call. Reserve returns the original
// record if the operation identity already exists; it must not replace it.
type Journal interface {
	Reserve(context.Context, Operation) (Operation, bool, error)
	Load(context.Context, Identity) (Operation, error)
	CompareAndSwap(context.Context, Operation, int) error
}

type Connector struct {
	resolve     Resolver
	authorize   Authorizer
	journal     Journal
	metricsRead func(context.Context, Target, string) (io.ReadCloser, error) // private deterministic connector-test seam; default is fixed Service proxy
}

func New(resolve Resolver, authorize Authorizer, journal Journal) *Connector {
	return &Connector{resolve: resolve, authorize: authorize, journal: journal}
}
