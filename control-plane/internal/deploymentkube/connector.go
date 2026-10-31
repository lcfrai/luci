package deploymentkube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ErrRefused = errors.New("The exact cluster plan, original actor, target or resource identity is unavailable or changed.")
var ErrUnknown = errors.New("The retained workload operation is uncertain. Observe its original identities; no mutation was replayed.")
var token = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

const operationAnnotation = "luci.ai/workload-operation"
const artifactAnnotation = "luci.ai/workload-artifact-sha256"

func digest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func projectLabel(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])[:12]
}
func operationLabel(id Identity) string { return digest(id) }

type desiredResource struct {
	kind, name, hash string
	deployment       *appsv1.Deployment
	service          *corev1.Service
}

func prepare(request Request) (Request, []desiredResource, error) {
	id := request.Identity
	if !token.MatchString(id.ProjectID) || !token.MatchString(id.ActorID) || id.TeamID != "" && !token.MatchString(id.TeamID) || !token.MatchString(id.TargetID) || !token.MatchString(id.OperationID) || !hashPattern.MatchString(id.PlanDigest) || !hashPattern.MatchString(id.TargetFingerprint) || request.ApprovalReference == "" || len(request.ApprovalReference) > 512 || request.Blueprint.ProjectID != id.ProjectID {
		return Request{}, nil, ErrRefused
	}
	plan, err := deploymentplan.Compile(request.Blueprint)
	if err != nil || plan.State != "compiled-prototype" || plan.Blueprint.Infrastructure.Kind != "kubernetes" || plan.Digest != id.PlanDigest || len(request.Expected) != len(plan.Artifacts) {
		return Request{}, nil, ErrRefused
	}
	request.Blueprint = plan.Blueprint
	if canonicalReferenceSnapshots(&request) != nil {
		return Request{}, nil, ErrRefused
	}
	expected := map[string]ExpectedResource{}
	for _, row := range request.Expected {
		key := row.Kind + "/" + row.Name
		if row.Kind != "Deployment" && row.Kind != "Service" || !token.MatchString(row.Name) || len(row.Name) > 63 || len(row.UID) > 128 || len(row.ResourceVersion) > 128 || (row.UID == "") != (row.ResourceVersion == "") {
			return Request{}, nil, ErrRefused
		}
		if _, duplicate := expected[key]; duplicate {
			return Request{}, nil, ErrRefused
		}
		expected[key] = row
	}
	resources := []desiredResource{}
	for _, artifact := range plan.Artifacts {
		var meta struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal([]byte(artifact.Content), &meta) != nil {
			return Request{}, nil, ErrRefused
		}
		row := desiredResource{kind: meta.Kind, hash: artifact.SHA256}
		if meta.Kind == "Deployment" {
			row.deployment = &appsv1.Deployment{}
			err = json.Unmarshal([]byte(artifact.Content), row.deployment)
			row.name = row.deployment.Name
		} else if meta.Kind == "Service" {
			row.service = &corev1.Service{}
			err = json.Unmarshal([]byte(artifact.Content), row.service)
			row.name = row.service.Name
		} else {
			return Request{}, nil, ErrRefused
		}
		if err != nil {
			return Request{}, nil, ErrRefused
		}
		if _, found := expected[row.kind+"/"+row.name]; !found {
			return Request{}, nil, ErrRefused
		}
		resources = append(resources, row)
	}
	// Stable expected order enters the durable request digest; caller ordering
	// cannot make a second request identity for the same reviewed resources.
	request.Expected = nil
	for _, row := range resources {
		request.Expected = append(request.Expected, expected[row.kind+"/"+row.name])
	}
	return request, resources, nil
}

func (c *Connector) Plan(ctx context.Context, b deploymentplan.Blueprint) (deploymentplan.Plan, error) {
	return (deploymentplan.ManifestPlanner{}).Plan(ctx, b)
}

func (c *Connector) target(ctx context.Context, request Request) (Target, error) {
	if c == nil || c.resolve == nil || c.authorize == nil || c.journal == nil || ctx.Err() != nil {
		return Target{}, ErrRefused
	}
	target, err := c.resolve(ctx, request.Identity)
	if err != nil || target.Client == nil || target.ID != request.Identity.TargetID || target.Fingerprint != request.Identity.TargetFingerprint || target.Namespace != request.Blueprint.Infrastructure.Namespace || target.NamespaceUID == "" {
		return Target{}, ErrRefused
	}
	namespace, err := target.Client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
	if err != nil || namespace == nil || string(namespace.UID) != target.NamespaceUID || namespace.DeletionTimestamp != nil {
		return Target{}, ErrRefused
	}
	return target, nil
}

func (c *Connector) save(ctx context.Context, operation *Operation, state string) error {
	before := operation.Revision
	operation.Revision++
	operation.State = state
	operation.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return c.journal.CompareAndSwap(ctx, *operation, before)
}

func objectOwned(meta metav1.Object, request Request, desired desiredResource, operation bool) bool {
	return meta.GetName() == desired.name && meta.GetNamespace() == request.Blueprint.Infrastructure.Namespace && meta.GetDeletionTimestamp() == nil &&
		meta.GetLabels()["app.kubernetes.io/managed-by"] == "luci-blueprint" && meta.GetLabels()["luci.ai/project"] == projectLabel(request.Identity.ProjectID) &&
		len(meta.GetOwnerReferences()) == 0 &&
		(!operation || meta.GetAnnotations()[operationAnnotation] == operationLabel(request.Identity) && meta.GetAnnotations()[artifactAnnotation] == desired.hash)
}

func stamp(meta metav1.Object, request Request, desired desiredResource) {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[operationAnnotation], annotations[artifactAnnotation] = operationLabel(request.Identity), desired.hash
	meta.SetAnnotations(annotations)
}

func preserveMetadata(object, previous metav1.Object) {
	labels, annotations := map[string]string{}, map[string]string{}
	for key, value := range previous.GetLabels() {
		labels[key] = value
	}
	for key, value := range object.GetLabels() {
		labels[key] = value
	}
	for key, value := range previous.GetAnnotations() {
		annotations[key] = value
	}
	for key, value := range object.GetAnnotations() {
		annotations[key] = value
	}
	object.SetLabels(labels)
	object.SetAnnotations(annotations)
	object.SetFinalizers(append([]string{}, previous.GetFinalizers()...))
}

// Apply never resumes an existing operation by mutation. An interrupted first
// attempt is observed, including unattempted remainder; a new reviewed action
// with fresh resource CAS expectations is required to do further work.
func (c *Connector) Apply(ctx context.Context, input Request) (Operation, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	request, resources, err := prepare(input)
	if err != nil || c == nil || c.authorize == nil || c.journal == nil {
		return Operation{}, ErrRefused
	}
	if err = c.authorize(ctx, request, true); err != nil {
		return Operation{}, ErrRefused
	}
	admissionTarget, err := c.target(ctx, request)
	if err != nil {
		return Operation{}, ErrRefused
	}
	if c.validateReferences(ctx, admissionTarget, request, nil, true) != nil {
		return Operation{}, ErrRefused
	}
	operation := Operation{Revision: 1, Request: request, RequestDigest: digest(request), NamespaceUID: admissionTarget.NamespaceUID, State: "reserved", Resources: []ResourceReceipt{}, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	for index, row := range resources {
		operation.Resources = append(operation.Resources, ResourceReceipt{ExpectedResource: request.Expected[index], DesiredSHA256: row.hash})
	}
	saved, fresh, err := c.journal.Reserve(ctx, operation)
	if err != nil {
		return Operation{}, err
	}
	if !fresh {
		if saved.RequestDigest != operation.RequestDigest {
			return Operation{}, ErrRefused
		}
		return c.Observe(ctx, request.Identity)
	}
	for index, desired := range resources {
		if c.authorize(ctx, request, true) != nil {
			return Operation{}, ErrRefused
		}
		target, targetErr := c.target(ctx, request)
		if targetErr != nil || target.NamespaceUID != operation.NamespaceUID {
			return Operation{}, ErrRefused
		}
		expected := request.Expected[index]
		var current metav1.Object
		if desired.kind == "Deployment" {
			current, err = target.Client.AppsV1().Deployments(target.Namespace).Get(ctx, desired.name, metav1.GetOptions{})
		} else {
			current, err = target.Client.CoreV1().Services(target.Namespace).Get(ctx, desired.name, metav1.GetOptions{})
		}
		if expected.UID == "" {
			if !apierrors.IsNotFound(err) {
				_ = c.save(ctx, &operation, "blocked")
				return operation, ErrRefused
			}
		} else if err != nil || current == nil || string(current.GetUID()) != expected.UID || current.GetResourceVersion() != expected.ResourceVersion || !objectOwned(current, request, desired, false) {
			_ = c.save(ctx, &operation, "blocked")
			return operation, ErrRefused
		}
		if err = c.authorize(ctx, request, true); err != nil {
			return Operation{}, ErrRefused
		}
		operation.Resources[index].Attempted = true
		if err = c.save(ctx, &operation, "unknown"); err != nil {
			return operation, err
		}
		// Journal/authority I/O may have waited. Re-resolve the concrete target
		// and original namespace after that wait; an observed replacement is refused.
		// Kubernetes Create has no namespace-UID precondition: this is not an
		// atomic guarantee against concurrent namespace replacement. Returned
		// object UIDs are retained and observation confirms the namespace again.
		target, err = c.target(ctx, request)
		if err != nil || target.NamespaceUID != operation.NamespaceUID {
			return Operation{}, ErrRefused
		}
		var component *deploymentplan.Service
		for serviceIndex := range request.Blueprint.Services {
			candidate := &request.Blueprint.Services[serviceIndex]
			labels := desired.service
			if desired.deployment != nil {
				if desired.deployment.Labels["luci.ai/component"] == candidate.ID {
					component = candidate
				}
			}
			if labels != nil && labels.Labels["luci.ai/component"] == candidate.ID {
				component = candidate
			}
		}
		if component == nil || c.validateReferences(ctx, target, request, component, true) != nil {
			return Operation{}, ErrRefused
		}
		// Fresh grant after target I/O and immediately before the only write.
		if err = c.authorize(ctx, request, true); err != nil {
			return Operation{}, ErrRefused
		}
		var written metav1.Object
		if desired.kind == "Deployment" {
			object := desired.deployment.DeepCopy()
			if expected.UID != "" {
				preserveMetadata(object, current)
			}
			stamp(object, request, desired)
			object.ResourceVersion = expected.ResourceVersion
			if expected.UID == "" {
				written, err = target.Client.AppsV1().Deployments(target.Namespace).Create(ctx, object, metav1.CreateOptions{})
			} else {
				object.UID = current.GetUID()
				written, err = target.Client.AppsV1().Deployments(target.Namespace).Update(ctx, object, metav1.UpdateOptions{})
			}
		} else {
			object := desired.service.DeepCopy()
			if expected.UID != "" {
				preserveMetadata(object, current)
			}
			stamp(object, request, desired)
			object.ResourceVersion = expected.ResourceVersion
			if expected.UID == "" {
				written, err = target.Client.CoreV1().Services(target.Namespace).Create(ctx, object, metav1.CreateOptions{})
			} else {
				old := current.(*corev1.Service)
				object.UID = old.UID
				object.Spec.ClusterIP, object.Spec.ClusterIPs, object.Spec.IPFamilies, object.Spec.IPFamilyPolicy = old.Spec.ClusterIP, old.Spec.ClusterIPs, old.Spec.IPFamilies, old.Spec.IPFamilyPolicy
				written, err = target.Client.CoreV1().Services(target.Namespace).Update(ctx, object, metav1.UpdateOptions{})
			}
		}
		if err != nil {
			return operation, ErrUnknown
		} // attempted marker already durable
		if written == nil || written.GetUID() == "" || written.GetResourceVersion() == "" || !objectOwned(written, request, desired, true) || expected.UID != "" && string(written.GetUID()) != expected.UID {
			return operation, ErrUnknown
		}
		operation.Resources[index].ObservedUID, operation.Resources[index].ObservedVersion = string(written.GetUID()), written.GetResourceVersion()
		if err = c.save(ctx, &operation, "pending"); err != nil {
			return operation, err
		}
	}
	return c.Observe(ctx, request.Identity)
}

func (c *Connector) Observe(ctx context.Context, identity Identity) (Operation, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if c == nil || c.authorize == nil || c.journal == nil {
		return Operation{}, ErrRefused
	}
	operation, err := c.journal.Load(ctx, identity)
	if err != nil || operation.Request.Identity != identity {
		return Operation{}, ErrRefused
	}
	request, resources, err := prepare(operation.Request)
	if err != nil || operation.RequestDigest != digest(request) || len(operation.Resources) != len(resources) || c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	target, err := c.target(ctx, request)
	if err != nil || target.NamespaceUID != operation.NamespaceUID {
		return Operation{}, ErrRefused
	}
	state := "ready"
	operation.ReferenceState = "current"
	if deploymentplan.RuntimeReferenceCount(request.Blueprint) > 0 && len(request.References) == 0 {
		operation.ReferenceState = "unreviewed"
		state = "unknown"
	} else if referenceErr := c.validateReferences(ctx, target, request, nil, false); referenceErr != nil {
		operation.ReferenceState = "unavailable"
		if errors.Is(referenceErr, ErrReferenceChanged) {
			operation.ReferenceState = "changed"
		}
		state = "unknown"
	}
	operation.Workloads = nil
	workloadBudget := &workloadObservationBudget{}
	for index, desired := range resources {
		if c.authorize(ctx, request, false) != nil {
			return Operation{}, ErrRefused
		}
		receipt := &operation.Resources[index]
		receipt.Ready = false
		if receipt.ExpectedResource != request.Expected[index] || receipt.DesiredSHA256 != desired.hash {
			return Operation{}, ErrRefused
		}
		if !receipt.Attempted {
			state = "unknown"
			continue
		}
		var meta metav1.Object
		if desired.kind == "Deployment" {
			object, readErr := target.Client.AppsV1().Deployments(target.Namespace).Get(ctx, desired.name, metav1.GetOptions{})
			err = readErr
			meta = object
			if err == nil && object != nil && objectOwned(object, request, desired, true) && deploymentMatches(desired.deployment, object) && (receipt.ObservedUID == "" || receipt.ObservedUID == string(object.UID)) && (receipt.UID == "" || receipt.UID == string(object.UID)) {
				receipt.ObservedGeneration = object.Generation
				replicas := *desired.deployment.Spec.Replicas
				receipt.Ready = object.Status.ObservedGeneration >= object.Generation && object.Status.Replicas == replicas && object.Status.UpdatedReplicas == replicas && object.Status.AvailableReplicas >= replicas && object.Status.ReadyReplicas >= replicas
				for _, service := range request.Blueprint.Services {
					if service.ID != object.Labels["luci.ai/component"] || service.Health == nil && service.Model == nil && len(service.Environment) == 0 {
						continue
					}
					observation := c.observeWorkload(ctx, target, request, service, object, workloadBudget)
					operation.Workloads = append(operation.Workloads, observation)
					if observation.State == "unknown" {
						state = "unknown"
						receipt.Ready = false
					} else if observation.ReadinessProbeConfigured && observation.State != "ready" || observation.ObservedPods != service.Replicas || observation.ReadyPods != service.Replicas {
						receipt.Ready = false
					}
				}
			} else {
				err = ErrUnknown
			}
		} else {
			object, readErr := target.Client.CoreV1().Services(target.Namespace).Get(ctx, desired.name, metav1.GetOptions{})
			err = readErr
			meta = object
			if err == nil && object != nil && objectOwned(object, request, desired, true) && serviceMatches(desired.service, object) {
				receipt.Ready = object.Spec.ClusterIP != "" && object.Spec.ClusterIP != "None"
			} else {
				err = ErrUnknown
			}
		}
		if err != nil || meta == nil || meta.GetUID() == "" || meta.GetResourceVersion() == "" || receipt.ObservedUID != "" && receipt.ObservedUID != string(meta.GetUID()) || receipt.UID != "" && receipt.UID != string(meta.GetUID()) {
			receipt.Ready = false
			state = "unknown"
			continue
		}
		receipt.ObservedUID, receipt.ObservedVersion = string(meta.GetUID()), meta.GetResourceVersion()
		if operation.ReferenceState != "current" {
			receipt.Ready = false
		}
		if !receipt.Ready && state == "ready" {
			state = "pending"
		}
	}
	if c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	latestTarget, err := c.target(ctx, request)
	if err != nil || latestTarget.NamespaceUID != operation.NamespaceUID {
		return Operation{}, ErrRefused
	}
	if operation.ReferenceState == "current" && c.validateReferences(ctx, latestTarget, request, nil, false) != nil {
		operation.ReferenceState = "unavailable"
		state = "unknown"
		for index := range operation.Resources {
			operation.Resources[index].Ready = false
		}
	}
	if operation.ReferenceState != "current" {
		for index := range operation.Workloads {
			operation.Workloads[index].State = "unknown"
		}
	}
	if err = c.save(ctx, &operation, state); err != nil {
		return Operation{}, err
	}
	latestTarget, err = c.target(ctx, request)
	if err != nil || latestTarget.NamespaceUID != operation.NamespaceUID {
		return Operation{}, ErrRefused
	}
	if operation.ReferenceState == "current" && c.validateReferences(ctx, latestTarget, request, nil, false) != nil {
		return Operation{}, ErrRefused
	}
	if c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	} // saved observation is retained, never rolled back
	return operation, nil
}

// ValidateOperation is the private journal's closed-record check. It compiles
// pure data only; it does not contact a cluster or confer any authorization.
func ValidateOperation(operation Operation) error {
	if validateTelemetryObservation(operation.Telemetry, operation.Request, operation.Resources) != nil {
		return ErrRefused
	}
	request, resources, err := prepare(operation.Request)
	if err != nil || operation.Revision < 1 || operation.Revision > 1_000_000_000 || operation.RequestDigest != digest(request) || operation.NamespaceUID == "" || len(operation.NamespaceUID) > 128 || operation.ApplicationHealthVerified || len(operation.Resources) != len(resources) {
		return ErrRefused
	}
	if operation.State != "reserved" && operation.State != "unknown" && operation.State != "pending" && operation.State != "ready" && operation.State != "blocked" {
		return ErrRefused
	}
	if _, err := time.Parse(time.RFC3339Nano, operation.ObservedAt); err != nil {
		return ErrRefused
	}
	for index, row := range operation.Resources {
		if row.ExpectedResource != request.Expected[index] || row.DesiredSHA256 != resources[index].hash || len(row.ObservedUID) > 128 || len(row.ObservedVersion) > 128 || (row.ObservedUID == "") != (row.ObservedVersion == "") || row.Ready && (!row.Attempted || row.ObservedUID == "") {
			return ErrRefused
		}
	}
	if operation.ReferenceState != "" && operation.ReferenceState != "current" && operation.ReferenceState != "changed" && operation.ReferenceState != "unavailable" && operation.ReferenceState != "unreviewed" {
		return ErrRefused
	}
	for _, reference := range request.References {
		if reference.NamespaceUID != operation.NamespaceUID {
			return ErrRefused
		}
	}
	if len(operation.Workloads) > len(request.Blueprint.Services) {
		return ErrRefused
	}
	seenWorkloads := map[string]bool{}
	for _, row := range operation.Workloads {
		if !validWorkloadObservation(row) || seenWorkloads[row.ComponentID] || operation.ReferenceState != "current" && operation.ReferenceState != "" && row.State != "unknown" {
			return ErrRefused
		}
		seenWorkloads[row.ComponentID] = true
		matched := false
		for index, resource := range resources {
			if resource.deployment != nil && resource.deployment.Labels["luci.ai/component"] == row.ComponentID && operation.Resources[index].ObservedUID == row.DeploymentUID {
				matched = true
			}
		}
		if !matched {
			return ErrRefused
		}
	}
	return nil
}
