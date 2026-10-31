package deploymentkube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const operationalLogBytes = 64 << 10

// OperationalPod is a current controller-owned identity, not a browser supplied
// name or permission to attach/exec. It deliberately omits events and env values.
type OperationalPod struct {
	ComponentID     string `json:"componentId"`
	Name            string `json:"name"`
	UID             string `json:"uid"`
	ResourceVersion string `json:"resourceVersion"`
	DeploymentUID   string `json:"deploymentUid"`
	ServiceUID      string `json:"serviceUid"`
	ReplicaSetUID   string `json:"replicaSetUid"`
	Phase           string `json:"phase"`
	Ready           bool   `json:"ready"`
}

type OperationalPods struct {
	Identity       Identity         `json:"identity"`
	ComponentID    string           `json:"componentId"`
	NamespaceUID   string           `json:"namespaceUid"`
	ObservedAt     string           `json:"observedAt"`
	Pods           []OperationalPod `json:"pods"`
	Omitted        bool             `json:"omitted"`
	EvidenceDigest string           `json:"evidenceDigest"`
}

type LogQuery struct {
	ComponentID  string `json:"componentId"`
	PodUID       string `json:"podUid"`
	TailLines    int64  `json:"tailLines"`
	SinceSeconds int64  `json:"sinceSeconds"`
	Previous     bool   `json:"previous"`
}

// Text is a sensitive, bounded read response. It is never included in an
// Operation, model context or automatic finding. Selection is a separate action.
type OperationalLogs struct {
	Identity       Identity       `json:"identity"`
	NamespaceUID   string         `json:"namespaceUid"`
	Pod            OperationalPod `json:"pod"`
	Query          LogQuery       `json:"query"`
	ObservedAt     string         `json:"observedAt"`
	State          string         `json:"state"`
	Text           string         `json:"text"`
	TextSHA256     string         `json:"textSha256"`
	LineCount      int            `json:"lineCount"`
	Truncated      bool           `json:"truncated"`
	EvidenceDigest string         `json:"evidenceDigest"`
}

func (c *Connector) operationsInput(ctx context.Context, identity Identity) (Operation, Target, []desiredResource, error) {
	if c == nil || c.journal == nil || c.authorize == nil {
		return Operation{}, Target{}, nil, ErrRefused
	}
	operation, err := c.journal.Load(ctx, identity)
	if err != nil || operation.Request.Identity != identity {
		return Operation{}, Target{}, nil, ErrRefused
	}
	request, resources, err := prepare(operation.Request)
	if err != nil || operation.RequestDigest != digest(request) || len(resources) != len(operation.Resources) || c.authorize(ctx, request, false) != nil {
		return Operation{}, Target{}, nil, ErrRefused
	}
	target, err := c.target(ctx, request)
	if err != nil || target.NamespaceUID != operation.NamespaceUID {
		return Operation{}, Target{}, nil, ErrRefused
	}
	return operation, target, resources, nil
}

// currentOperationalPods includes waiting/terminated containers so a crash is
// inspectable. Label selectors only bound enumeration; original receipt UIDs,
// exact current specs and controller UID lineage establish attribution.
func (c *Connector) currentOperationalPods(ctx context.Context, operation Operation, target Target, resources []desiredResource, componentID string, budget *int) (OperationalPods, error) {
	result := OperationalPods{Identity: operation.Request.Identity, ComponentID: componentID, NamespaceUID: operation.NamespaceUID, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Pods: []OperationalPod{}}
	found := false
	for _, service := range operation.Request.Blueprint.Services {
		if service.ID != componentID {
			continue
		}
		found = true
		name := deploymentplan.ResourceName(operation.Request.Blueprint.ProjectID, service.ID)
		var desired *desiredResource
		var receipt *ResourceReceipt
		for i := range resources {
			if resources[i].kind == "Deployment" && resources[i].name == name {
				desired = &resources[i]
				receipt = &operation.Resources[i]
				break
			}
		}
		if desired == nil || receipt == nil || !receipt.Attempted || receipt.ObservedUID == "" || c.authorize(ctx, operation.Request, false) != nil {
			return result, ErrRefused
		}
		deployment, err := target.Client.AppsV1().Deployments(target.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil || deployment == nil || string(deployment.UID) != receipt.ObservedUID || !objectOwned(deployment, operation.Request, *desired, true) || !deploymentMatches(desired.deployment, deployment) || c.authorize(ctx, operation.Request, false) != nil {
			return result, ErrRefused
		}
		serviceUID := ""
		for i := range resources {
			if resources[i].kind != "Service" || resources[i].name != name {
				continue
			}
			original := operation.Resources[i]
			if !original.Attempted || original.ObservedUID == "" || c.authorize(ctx, operation.Request, false) != nil {
				return result, ErrRefused
			}
			actual, readErr := target.Client.CoreV1().Services(target.Namespace).Get(ctx, name, metav1.GetOptions{})
			if readErr != nil || actual == nil || string(actual.UID) != original.ObservedUID || !objectOwned(actual, operation.Request, resources[i], true) || !serviceMatches(resources[i].service, actual) || c.authorize(ctx, operation.Request, false) != nil {
				return result, ErrRefused
			}
			serviceUID = string(actual.UID)
		}
		if *budget >= 128 {
			result.Omitted = true
			break
		}
		selector := labels.SelectorFromSet(desired.deployment.Spec.Selector.MatchLabels).String()
		sets, err := target.Client.AppsV1().ReplicaSets(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector, Limit: int64(128 - *budget)})
		if err != nil || sets == nil || len(sets.Items) > 128-*budget || c.authorize(ctx, operation.Request, false) != nil {
			return result, ErrRefused
		}
		*budget += len(sets.Items)
		if sets.Continue != "" {
			result.Omitted = true
			continue
		}
		owned := map[string]*appsv1.ReplicaSet{}
		for i := range sets.Items {
			if currentReplicaSet(deployment, &sets.Items[i]) {
				if len(sets.Items[i].UID) > 128 || sets.Items[i].ResourceVersion == "" || len(sets.Items[i].ResourceVersion) > 128 {
					return result, ErrRefused
				}
				owned[string(sets.Items[i].UID)] = &sets.Items[i]
			}
		}
		if *budget >= 128 {
			result.Omitted = true
			break
		}
		pods, err := target.Client.CoreV1().Pods(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector, Limit: int64(128 - *budget)})
		if err != nil || pods == nil || len(pods.Items) > 128-*budget || c.authorize(ctx, operation.Request, false) != nil {
			return result, ErrRefused
		}
		*budget += len(pods.Items)
		if pods.Continue != "" {
			result.Omitted = true
			continue
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			var owner *appsv1.ReplicaSet
			for uid, set := range owned {
				if controllerOwner(pod, "ReplicaSet", uid) {
					owner = set
					break
				}
			}
			if owner == nil || pod.DeletionTimestamp != nil {
				continue
			}
			if pod.Namespace != target.Namespace || pod.UID == "" || pod.ResourceVersion == "" || len(pod.ResourceVersion) > 128 || len(pod.UID) > 128 || len(pod.Name) > 253 || !podMatchesTemplate(owner.Spec.Template.Spec, pod.Spec) || len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Name != service.ID {
				return result, ErrRefused
			}
			ready := false
			for _, condition := range pod.Status.Conditions {
				ready = ready || condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue
			}
			phase := "unknown"
			switch pod.Status.Phase {
			case corev1.PodPending:
				phase = "pending"
			case corev1.PodRunning:
				phase = "running"
			case corev1.PodSucceeded:
				phase = "succeeded"
			case corev1.PodFailed:
				phase = "failed"
			}
			result.Pods = append(result.Pods, OperationalPod{ComponentID: service.ID, Name: pod.Name, UID: string(pod.UID), ResourceVersion: pod.ResourceVersion, DeploymentUID: string(deployment.UID), ServiceUID: serviceUID, ReplicaSetUID: string(owner.UID), Phase: phase, Ready: ready})
		}
	}
	if !found {
		return result, ErrRefused
	}
	sort.Slice(result.Pods, func(i, j int) bool {
		if result.Pods[i].ComponentID != result.Pods[j].ComponentID {
			return result.Pods[i].ComponentID < result.Pods[j].ComponentID
		}
		return result.Pods[i].UID < result.Pods[j].UID
	})
	seen := map[string]bool{}
	for _, pod := range result.Pods {
		if seen[pod.UID] {
			return result, ErrRefused
		}
		seen[pod.UID] = true
	}
	result.EvidenceDigest = digest(result)
	if ctx.Err() != nil || c.authorize(ctx, operation.Request, false) != nil {
		return result, ErrRefused
	}
	return result, nil
}

func (c *Connector) ReadOperationalPods(ctx context.Context, identity Identity, componentID string) (OperationalPods, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	operation, target, resources, err := c.operationsInput(ctx, identity)
	if err != nil {
		return OperationalPods{}, ErrRefused
	}
	budget := 0
	view, err := c.currentOperationalPods(ctx, operation, target, resources, componentID, &budget)
	if err != nil {
		return OperationalPods{}, ErrRefused
	}
	latest, err := c.target(ctx, operation.Request)
	if err != nil || latest.NamespaceUID != operation.NamespaceUID || c.authorize(ctx, operation.Request, false) != nil {
		return OperationalPods{}, ErrRefused
	}
	return view, nil
}

func (c *Connector) ReadOperationalLogs(ctx context.Context, identity Identity, query LogQuery) (OperationalLogs, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if query.ComponentID == "" || query.PodUID == "" || query.TailLines < 1 || query.TailLines > 200 || query.SinceSeconds < 1 || query.SinceSeconds > 3600 {
		return OperationalLogs{}, ErrRefused
	}
	operation, target, resources, err := c.operationsInput(ctx, identity)
	if err != nil {
		return OperationalLogs{}, ErrRefused
	}
	budget := 0
	before, err := c.currentOperationalPods(ctx, operation, target, resources, query.ComponentID, &budget)
	if err != nil {
		return OperationalLogs{}, ErrRefused
	}
	var selected *OperationalPod
	for i := range before.Pods {
		if before.Pods[i].UID == query.PodUID && before.Pods[i].ComponentID == query.ComponentID {
			selected = &before.Pods[i]
		}
	}
	if selected == nil || c.authorize(ctx, operation.Request, false) != nil {
		return OperationalLogs{}, ErrRefused
	}
	streamCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	limitBytes := int64(operationalLogBytes + 1)
	stream, err := target.Client.CoreV1().Pods(target.Namespace).GetLogs(selected.Name, &corev1.PodLogOptions{Container: query.ComponentID, Follow: false, Previous: query.Previous, Timestamps: true, TailLines: &query.TailLines, SinceSeconds: &query.SinceSeconds, LimitBytes: &limitBytes}).Stream(streamCtx)
	if err != nil || stream == nil {
		stop()
		return OperationalLogs{}, ErrRefused
	}
	raw, readErr := io.ReadAll(io.LimitReader(stream, operationalLogBytes+1))
	closeErr := stream.Close()
	expired := streamCtx.Err()
	stop()
	if readErr != nil || closeErr != nil || expired != nil {
		return OperationalLogs{}, ErrRefused
	}
	truncated := len(raw) > operationalLogBytes
	if truncated {
		raw = raw[:operationalLogBytes]
		for len(raw) > 0 && !utf8.Valid(raw) {
			raw = raw[:len(raw)-1]
			if len(raw) < operationalLogBytes-3 {
				return OperationalLogs{}, ErrRefused
			}
		}
	}
	if !utf8.Valid(raw) {
		return OperationalLogs{}, ErrRefused
	}
	text := strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' || r == 127 {
			return -1
		}
		return r
	}, string(raw))
	latest, err := c.target(ctx, operation.Request)
	if err != nil || latest.NamespaceUID != operation.NamespaceUID {
		return OperationalLogs{}, ErrRefused
	}
	after, err := c.currentOperationalPods(ctx, operation, latest, resources, query.ComponentID, &budget)
	if err != nil {
		return OperationalLogs{}, ErrRefused
	}
	stable := false
	for _, pod := range after.Pods {
		if pod == *selected {
			stable = true
		}
	}
	if !stable || c.authorize(ctx, operation.Request, false) != nil || ctx.Err() != nil {
		return OperationalLogs{}, ErrRefused
	}
	sum := sha256.Sum256([]byte(text))
	state := "recorded"
	if text == "" {
		state = "empty"
	}
	if truncated {
		state = "partial"
	}
	lines := strings.Count(text, "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		lines++
	}
	value := OperationalLogs{Identity: identity, NamespaceUID: operation.NamespaceUID, Pod: *selected, Query: query, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), State: state, Text: text, TextSHA256: hex.EncodeToString(sum[:]), LineCount: lines, Truncated: truncated}
	metadata := value
	metadata.Text = ""
	value.EvidenceDigest = digest(metadata)
	return value, nil
}
