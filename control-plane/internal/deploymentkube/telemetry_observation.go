package deploymentkube

import (
	"context"
	"io"
	"net"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type telemetryRuntime struct {
	collector    deploymentplan.Service
	service      *corev1.Service
	deployment   *appsv1.Deployment
	pod          *corev1.Pod
	instances    map[string]string
	instanceUIDs []string
	fingerprint  string
	omitted      bool
}
type telemetryComponent struct {
	service     *corev1.Service
	deployment  *appsv1.Deployment
	pods        []*corev1.Pod
	fingerprint string
	omitted     bool
}

// These reads confirm the entire desired specs and the exact original operation
// UIDs. Labels only narrow List, never establish ownership. One shared bounded
// object budget spans ALL selected components and endpoint slices.
func (c *Connector) telemetryComponent(ctx context.Context, target Target, operation Operation, resources []desiredResource, service deploymentplan.Service, budget *int) (telemetryComponent, error) {
	var desiredDeployment, desiredService *desiredResource
	var receiptDeployment, receiptService *ResourceReceipt
	name := deploymentplan.ResourceName(operation.Request.Blueprint.ProjectID, service.ID)
	for i := range resources {
		resource := &resources[i]
		if resource.name != name {
			continue
		}
		if resource.kind == "Deployment" {
			desiredDeployment = resource
			receiptDeployment = &operation.Resources[i]
		} else if resource.kind == "Service" {
			desiredService = resource
			receiptService = &operation.Resources[i]
		}
	}
	if desiredDeployment == nil || desiredService == nil || !receiptDeployment.Attempted || !receiptService.Attempted || receiptDeployment.ObservedUID == "" || receiptService.ObservedUID == "" || c.authorize(ctx, operation.Request, false) != nil {
		return telemetryComponent{}, ErrRefused
	}
	deployment, err := target.Client.AppsV1().Deployments(target.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil || deployment == nil || string(deployment.UID) != receiptDeployment.ObservedUID || !objectOwned(deployment, operation.Request, *desiredDeployment, true) || !deploymentMatches(desiredDeployment.deployment, deployment) || c.authorize(ctx, operation.Request, false) != nil {
		return telemetryComponent{}, ErrRefused
	}
	actualService, err := target.Client.CoreV1().Services(target.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil || actualService == nil || string(actualService.UID) != receiptService.ObservedUID || !objectOwned(actualService, operation.Request, *desiredService, true) || !serviceMatches(desiredService.service, actualService) || actualService.Spec.ClusterIP == "" || actualService.Spec.ClusterIP == "None" || c.authorize(ctx, operation.Request, false) != nil {
		return telemetryComponent{}, ErrRefused
	}
	if *budget >= 128 {
		return telemetryComponent{}, ErrRefused
	}
	selector := labels.SelectorFromSet(desiredDeployment.deployment.Spec.Selector.MatchLabels).String()
	sets, err := target.Client.AppsV1().ReplicaSets(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector, Limit: int64(128 - *budget)})
	if err != nil || sets == nil || sets.Continue != "" || len(sets.Items) > 128-*budget || c.authorize(ctx, operation.Request, false) != nil {
		return telemetryComponent{}, ErrRefused
	}
	*budget += len(sets.Items)
	owned := map[string]*appsv1.ReplicaSet{}
	for i := range sets.Items {
		if currentReplicaSet(deployment, &sets.Items[i]) {
			owned[string(sets.Items[i].UID)] = &sets.Items[i]
		}
	}
	if len(owned) == 0 || *budget >= 128 {
		return telemetryComponent{}, ErrRefused
	}
	pods, err := target.Client.CoreV1().Pods(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector, Limit: int64(128 - *budget)})
	if err != nil || pods == nil || pods.Continue != "" || len(pods.Items) > 128-*budget || c.authorize(ctx, operation.Request, false) != nil {
		return telemetryComponent{}, ErrRefused
	}
	*budget += len(pods.Items)
	result := telemetryComponent{service: actualService, deployment: deployment, pods: []*corev1.Pod{}}
	identities := []string{string(deployment.UID), deployment.ResourceVersion, string(actualService.UID), actualService.ResourceVersion}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp != nil {
			continue
		}
		var owner *appsv1.ReplicaSet
		for uid, set := range owned {
			if controllerOwner(pod, "ReplicaSet", uid) {
				owner = set
				break
			}
		}
		if owner == nil {
			continue
		}
		if pod.Namespace != target.Namespace || pod.Name == "" || pod.UID == "" || len(pod.UID) > 128 || pod.CreationTimestamp.IsZero() || !podMatchesTemplate(owner.Spec.Template.Spec, pod.Spec) || len(pod.Status.ContainerStatuses) != 1 || pod.Status.ContainerStatuses[0].Name != service.ID {
			return telemetryComponent{}, ErrRefused
		}
		if pod.Status.Phase != corev1.PodRunning || pod.Status.ContainerStatuses[0].State.Running == nil {
			result.omitted = true
			continue
		}
		ready := false
		for _, condition := range pod.Status.Conditions {
			ready = ready || condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue
		}
		if !ready || !pod.Status.ContainerStatuses[0].Ready {
			result.omitted = true
		}
		result.pods = append(result.pods, pod)
		identities = append(identities, string(owner.UID), owner.ResourceVersion, string(pod.UID), pod.ResourceVersion)
	}
	if len(result.pods) != service.Replicas {
		result.omitted = true
	}
	sort.Strings(identities)
	result.fingerprint = digest(identities)
	if ctx.Err() != nil || c.authorize(ctx, operation.Request, false) != nil {
		return telemetryComponent{}, ErrRefused
	}
	return result, nil
}

func (c *Connector) telemetryRuntime(ctx context.Context, target Target, operation Operation, resources []desiredResource, budget *int) (telemetryRuntime, error) {
	result := telemetryRuntime{instances: map[string]string{}, instanceUIDs: []string{}}
	identities := []string{}
	for _, service := range operation.Request.Blueprint.Services {
		if service.Collector == nil && (service.Telemetry == nil || deploymentplan.ManagedCollector(operation.Request.Blueprint, service.Telemetry.ResourceID) == nil) {
			continue
		}
		component, err := c.telemetryComponent(ctx, target, operation, resources, service, budget)
		if err != nil {
			return result, err
		}
		identities = append(identities, component.fingerprint)
		result.omitted = result.omitted || component.omitted
		if service.Collector != nil {
			if result.service != nil || len(component.pods) != 1 || component.omitted {
				return result, ErrRefused
			}
			pod := component.pods[0]
			ready := false
			for _, condition := range pod.Status.Conditions {
				ready = ready || condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue
			}
			if !ready || !pod.Status.ContainerStatuses[0].Ready {
				return result, ErrRefused
			}
			result.collector, result.service, result.deployment, result.pod = service, component.service, component.deployment, pod
		} else {
			for _, pod := range component.pods {
				uid := string(pod.UID)
				if _, exists := result.instances[uid]; exists {
					return result, ErrRefused
				}
				result.instances[uid] = service.ID
				result.instanceUIDs = append(result.instanceUIDs, uid)
			}
		}
	}
	if result.service == nil || result.pod == nil || *budget >= 128 {
		return result, ErrRefused
	}
	slices, err := target.Client.DiscoveryV1().EndpointSlices(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"kubernetes.io/service-name": result.service.Name}).String(), Limit: int64(128 - *budget)})
	if err != nil || slices == nil || slices.Continue != "" || len(slices.Items) == 0 || len(slices.Items) > 4 || len(slices.Items) > 128-*budget || c.authorize(ctx, operation.Request, false) != nil {
		return result, ErrRefused
	}
	*budget += len(slices.Items)
	ipAddresses := map[string]bool{}
	for _, ip := range result.pod.Status.PodIPs {
		if net.ParseIP(ip.IP) == nil {
			return result, ErrRefused
		}
		ipAddresses[ip.IP] = true
	}
	if len(ipAddresses) == 0 {
		return result, ErrRefused
	}
	// The API server's Service proxy currently chooses core/v1 Endpoints, not
	// EndpointSlices. Confirm that exact routing surface too; labels alone or
	// a matching Slice never authorize a proxy to a different Pod.
	legacy, err := target.Client.CoreV1().Endpoints(target.Namespace).Get(ctx, result.service.Name, metav1.GetOptions{})
	if err != nil || legacy == nil || legacy.Name != result.service.Name || legacy.Namespace != target.Namespace || legacy.UID == "" || legacy.ResourceVersion == "" || legacy.DeletionTimestamp != nil || len(legacy.Subsets) < 1 || len(legacy.Subsets) > 4 || legacy.Annotations["endpoints.kubernetes.io/over-capacity"] != "" || c.authorize(ctx, operation.Request, false) != nil {
		return result, ErrRefused
	}
	seenAddresses := map[string]bool{}
	for _, subset := range legacy.Subsets {
		if len(subset.NotReadyAddresses) != 0 || len(subset.Addresses) < 1 || len(subset.Addresses) > 2 || len(subset.Ports) != 2 {
			return result, ErrRefused
		}
		ports := map[string]bool{}
		for _, port := range subset.Ports {
			if ports[port.Name] || port.Protocol != corev1.ProtocolTCP || (port.Name != "service" || port.Port != 4318) && (port.Name != "metrics" || port.Port != deploymentplan.CollectorMetricsPort) {
				return result, ErrRefused
			}
			ports[port.Name] = true
		}
		if !ports["metrics"] || !ports["service"] {
			return result, ErrRefused
		}
		for _, address := range subset.Addresses {
			ref := address.TargetRef
			if !ipAddresses[address.IP] || seenAddresses[address.IP] || ref == nil || ref.APIVersion != "" && ref.APIVersion != "v1" || ref.Kind != "Pod" || ref.Namespace != target.Namespace || ref.Name != result.pod.Name || ref.UID != result.pod.UID {
				return result, ErrRefused
			}
			seenAddresses[address.IP] = true
		}
	}
	identities = append(identities, string(legacy.UID), legacy.ResourceVersion, digest(legacy.Subsets))
	endpoints := 0
	for _, slice := range slices.Items {
		if len(slice.Endpoints) > 2 || len(slice.Ports) > 2 || len(slice.OwnerReferences) > 8 {
			return result, ErrRefused
		}
		if slice.Namespace != target.Namespace || slice.UID == "" || slice.ResourceVersion == "" || slice.DeletionTimestamp != nil {
			return result, ErrRefused
		}
		owner := false
		for _, ref := range slice.OwnerReferences {
			if ref.Controller == nil || !*ref.Controller {
				continue
			}
			if owner || ref.APIVersion != "v1" || ref.Kind != "Service" || ref.UID != result.service.UID {
				return result, ErrRefused
			}
			owner = true
		}
		if !owner {
			return result, ErrRefused
		}
		metricsPort := false
		for _, port := range slice.Ports {
			if port.Name != nil && *port.Name == "metrics" {
				if metricsPort || port.Port == nil || *port.Port != deploymentplan.CollectorMetricsPort || port.Protocol == nil || *port.Protocol != corev1.ProtocolTCP {
					return result, ErrRefused
				}
				metricsPort = true
			}
		}
		if !metricsPort {
			return result, ErrRefused
		}
		for _, endpoint := range slice.Endpoints {
			if endpoint.Conditions.Ready == nil || !*endpoint.Conditions.Ready || endpoint.Conditions.Terminating != nil && *endpoint.Conditions.Terminating {
				return result, ErrRefused
			}
			ref := endpoint.TargetRef
			if ref == nil || ref.APIVersion != "" && ref.APIVersion != "v1" || ref.Kind != "Pod" || ref.Namespace != target.Namespace || ref.Name != result.pod.Name || ref.UID != result.pod.UID || len(endpoint.Addresses) == 0 || len(endpoint.Addresses) > 2 {
				return result, ErrRefused
			}
			for _, ip := range endpoint.Addresses {
				if !ipAddresses[ip] {
					return result, ErrRefused
				}
			}
			endpoints++
		}
		identities = append(identities, string(slice.UID), slice.ResourceVersion)
	}
	if endpoints == 0 {
		return result, ErrRefused
	}
	sort.Strings(result.instanceUIDs)
	sort.Strings(identities)
	result.fingerprint = digest(identities)
	if ctx.Err() != nil || c.authorize(ctx, operation.Request, false) != nil {
		return result, ErrRefused
	}
	return result, nil
}

// ReadTelemetry is an EXPLICIT bounded observation, not part of ordinary GET,
// Apply, background polling or scheduler admission. It uses only the configured
// Kubernetes client's one fixed owned Service proxy path. No arbitrary URL,
// pod exec/port-forward, raw telemetry or backend credentials enter Luci.
func (c *Connector) ReadTelemetry(ctx context.Context, identity Identity) (Operation, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
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
	if err != nil || target.NamespaceUID != operation.NamespaceUID || c.validateReferences(ctx, target, request, nil, false) != nil {
		return Operation{}, ErrRefused
	}
	budget := 0
	before, err := c.telemetryRuntime(ctx, target, operation, resources, &budget)
	if err != nil {
		return Operation{}, ErrRefused
	}
	if c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	var stream io.ReadCloser
	streamContext, stopStream := context.WithTimeout(ctx, 5*time.Second)
	if c.metricsRead != nil {
		stream, err = c.metricsRead(streamContext, target, before.service.Name)
	} else {
		stream, err = target.Client.CoreV1().Services(target.Namespace).ProxyGet("http", before.service.Name, "metrics", "metrics", nil).Stream(streamContext)
	}
	if err != nil || stream == nil {
		stopStream()
		return Operation{}, ErrRefused
	}
	raw, readErr := io.ReadAll(io.LimitReader(stream, telemetryBodyLimit+1))
	closeErr := stream.Close()
	streamExpired := streamContext.Err()
	stopStream()
	if readErr != nil || closeErr != nil || streamExpired != nil || len(raw) > telemetryBodyLimit {
		return Operation{}, ErrRefused
	}
	samples, omitted, err := parseProductMetrics(raw, request, before.collector, before.instances)
	if err != nil {
		return Operation{}, ErrRefused
	}
	if c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	latestTarget, err := c.target(ctx, request)
	if err != nil || latestTarget.NamespaceUID != operation.NamespaceUID || c.validateReferences(ctx, latestTarget, request, nil, false) != nil {
		return Operation{}, ErrRefused
	}
	after, err := c.telemetryRuntime(ctx, latestTarget, operation, resources, &budget)
	if err != nil || after.fingerprint != before.fingerprint || c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	observation := TelemetryObservation{State: "recorded", CollectorID: before.collector.ID, ServiceUID: string(before.service.UID), DeploymentUID: string(before.deployment.UID), CollectorPodUID: string(before.pod.UID), ApplicationPodUIDs: before.instanceUIDs, PlanDigest: identity.PlanDigest, SourceCommit: request.Blueprint.Source.Commit, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Samples: samples, Omitted: omitted || before.omitted}
	if observation.Omitted {
		observation.State = "partial"
	} else if len(samples) == 0 {
		observation.State = "no-data"
	}
	observation.Continuity = telemetryContinuity(operation.Telemetry, observation)
	observation.EvidenceDigest = telemetryDigest(observation)
	if validateTelemetryObservation(&observation, request, operation.Resources) != nil {
		return Operation{}, ErrRefused
	}
	operation.Telemetry = &observation
	if c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	if err = c.save(ctx, &operation, operation.State); err != nil {
		return Operation{}, err
	}
	if c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	latestTarget, err = c.target(ctx, request)
	if err != nil || latestTarget.NamespaceUID != operation.NamespaceUID || c.validateReferences(ctx, latestTarget, request, nil, false) != nil || c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	return operation, nil
}
