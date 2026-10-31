package deploymentkube

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

type memoryJournal struct {
	operation *Operation
	failCAS   bool
	afterSave func()
}

func cloneOperation(value Operation) Operation {
	raw, _ := json.Marshal(value)
	var copy Operation
	_ = json.Unmarshal(raw, &copy)
	return copy
}
func (m *memoryJournal) Reserve(_ context.Context, op Operation) (Operation, bool, error) {
	if m.operation != nil {
		return cloneOperation(*m.operation), false, nil
	}
	copy := cloneOperation(op)
	m.operation = &copy
	return cloneOperation(copy), true, nil
}
func (m *memoryJournal) Load(context.Context, Identity) (Operation, error) {
	if m.operation == nil {
		return Operation{}, ErrRefused
	}
	return cloneOperation(*m.operation), nil
}
func (m *memoryJournal) CompareAndSwap(_ context.Context, op Operation, revision int) error {
	if m.failCAS || m.operation == nil || m.operation.Revision != revision {
		return ErrRefused
	}
	copy := cloneOperation(op)
	m.operation = &copy
	if m.afterSave != nil {
		m.afterSave()
	}
	return nil
}

func workloadFixture(t *testing.T) (*Connector, *fake.Clientset, *memoryJournal, Request) {
	t.Helper()
	b := deploymentplan.Blueprint{Version: 1, ProjectID: "p-one", Source: deploymentplan.Source{Commit: strings.Repeat("a", 40)},
		Infrastructure: deploymentplan.Infrastructure{ID: "cluster", Kind: "kubernetes", Reference: "owned-cluster", Namespace: "apps"},
		Services:       []deploymentplan.Service{{ID: "web", Kind: "app", Image: "registry.example/web@sha256:" + strings.Repeat("b", 64), Port: 8080, Replicas: 1, Resources: deploymentplan.ResourceBudget{RequestMilliCPU: 100, LimitMilliCPU: 500, RequestMemoryMiB: 128, LimitMemoryMiB: 512}}}}
	plan, err := deploymentplan.Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Identity: Identity{ProjectID: b.ProjectID, ActorID: "alice", TargetID: "cluster-one", TargetFingerprint: strings.Repeat("c", 64), OperationID: "workload-one", PlanDigest: plan.Digest}, Blueprint: b, ApprovalReference: "controller-reviewed-approval", Expected: []ExpectedResource{}}
	for _, artifact := range plan.Artifacts {
		var meta struct {
			Kind     string
			Metadata struct{ Name string }
		}
		if json.Unmarshal([]byte(artifact.Content), &meta) != nil {
			t.Fatal("bad compiler artifact")
		}
		request.Expected = append(request.Expected, ExpectedResource{Kind: meta.Kind, Name: meta.Metadata.Name})
	}
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "apps", UID: "namespace-one"}})
	client.PrependReactor("create", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
		object := action.(ktesting.CreateAction).GetObject().DeepCopyObject()
		meta := object.(metav1.Object)
		meta.SetUID(types.UID("uid-" + meta.GetName() + "-" + action.GetResource().Resource))
		meta.SetResourceVersion("1")
		meta.SetGeneration(1)
		if service, ok := object.(*corev1.Service); ok {
			service.Spec.ClusterIP = "10.0.0.2"
		}
		if err := client.Tracker().Create(action.GetResource(), object, action.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, object, nil
	})
	journal := &memoryJournal{}
	connector := New(func(context.Context, Identity) (Target, error) {
		return Target{ID: "cluster-one", Fingerprint: strings.Repeat("c", 64), Namespace: "apps", NamespaceUID: "namespace-one", Client: client}, nil
	},
		func(context.Context, Request, bool) error { return nil }, journal)
	return connector, client, journal, request
}

func writes(client *fake.Clientset) int {
	count := 0
	for _, action := range client.Actions() {
		if action.GetVerb() == "create" || action.GetVerb() == "update" || action.GetVerb() == "patch" || action.GetVerb() == "delete" {
			count++
		}
	}
	return count
}

func TestWorkloadApplyReservesWritesAndObservesReplicaReadinessWithoutApplicationHealthClaim(t *testing.T) {
	c, client, journal, request := workloadFixture(t)
	op, err := c.Apply(context.Background(), request)
	if err != nil || op.State != "pending" || writes(client) != 2 || op.ApplicationHealthVerified || journal.operation == nil {
		t.Fatalf("unexpected apply %+v %v", op, err)
	}
	deploymentName := request.Expected[0].Name
	deployment, err := client.AppsV1().Deployments("apps").Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	deployment.Status = appsv1.DeploymentStatus{ObservedGeneration: 1, Replicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1, ReadyReplicas: 1}
	if _, err = client.AppsV1().Deployments("apps").UpdateStatus(context.Background(), deployment, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	op, err = c.Observe(context.Background(), request.Identity)
	if err != nil || op.State != "ready" || op.ApplicationHealthVerified || op.Resources[0].ObservedUID == "" || op.NamespaceUID != "namespace-one" {
		t.Fatalf("readiness not observed %+v %v", op, err)
	}
}

func TestWorkloadLostCreateAcknowledgementReconcilesOriginalOperationWithoutReplay(t *testing.T) {
	c, client, journal, request := workloadFixture(t)
	client.PrependReactor("create", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
		object := action.(ktesting.CreateAction).GetObject().(*appsv1.Deployment).DeepCopy()
		object.UID = "created-before-lost-ack"
		object.ResourceVersion = "1"
		object.Generation = 1
		if err := client.Tracker().Create(action.GetResource(), object, "apps"); err != nil {
			t.Fatal(err)
		}
		return true, nil, errors.New("synthetic lost acknowledgement")
	})
	if _, err := c.Apply(context.Background(), request); !errors.Is(err, ErrUnknown) {
		t.Fatal("lost response did not remain unknown", err)
	}
	if !journal.operation.Resources[0].Attempted {
		t.Fatal("attempt was not durable before write")
	}
	op, err := c.Apply(context.Background(), request)
	if err != nil || op.State != "unknown" || op.Resources[0].ObservedUID != "created-before-lost-ack" || writes(client) != 1 {
		t.Fatalf("operation replayed or lost identity %+v %v", op, err)
	}
}

func TestWorkloadCurrentGrantAndJournalFailureFenceEveryMutation(t *testing.T) {
	c, client, journal, request := workloadFixture(t)
	allowed := true
	c.authorize = func(context.Context, Request, bool) error {
		if !allowed {
			return ErrRefused
		}
		return nil
	}
	client.PrependReactor("get", "deployments", func(ktesting.Action) (bool, runtime.Object, error) { allowed = false; return false, nil, nil })
	if op, err := c.Apply(context.Background(), request); err == nil || writes(client) != 0 || op.Request.Identity.ProjectID != "" {
		t.Fatal("revocation allowed a mutation or disclosed private body", err)
	}
	if journal.operation == nil || journal.operation.Resources[0].Attempted {
		t.Fatal("refused mutation was incorrectly marked attempted")
	}
	c, client, journal, request = workloadFixture(t)
	journal.failCAS = true
	if _, err := c.Apply(context.Background(), request); err == nil || writes(client) != 0 {
		t.Fatal("journal failure allowed a provider write", err)
	}
}

func TestWorkloadNamespaceReplacementDuringJournalWaitRefusesWriteAndRetainsAttempt(t *testing.T) {
	c, client, journal, request := workloadFixture(t)
	journal.afterSave = func() {
		journal.afterSave = nil
		namespace, err := client.CoreV1().Namespaces().Get(context.Background(), "apps", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		namespace.UID = "replacement-during-journal"
		if _, err = client.CoreV1().Namespaces().Update(context.Background(), namespace, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Apply(context.Background(), request); !errors.Is(err, ErrRefused) {
		t.Fatal("replacement namespace was used after journal wait", err)
	}
	for _, action := range client.Actions() {
		if action.Matches("create", "deployments") || action.Matches("create", "services") {
			t.Fatal("write reached replacement namespace")
		}
	}
	if journal.operation == nil || !journal.operation.Resources[0].Attempted {
		t.Fatal("attempt fence was removed after refusal")
	}
}

func TestWorkloadForeignObjectsAndStaleReviewedResourceVersionAreNotAdopted(t *testing.T) {
	c, client, _, request := workloadFixture(t)
	foreign := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: request.Expected[0].Name, Namespace: "apps", UID: "foreign", ResourceVersion: "1"}}
	if err := client.Tracker().Create(appsv1.SchemeGroupVersion.WithResource("deployments"), foreign, "apps"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Apply(context.Background(), request); err == nil || writes(client) != 0 {
		t.Fatal("reviewed absence adopted an existing foreign workload", err)
	}
	c, client, _, request = workloadFixture(t)
	_, desired, err := prepare(request)
	if err != nil {
		t.Fatal(err)
	}
	owned := desired[0].deployment.DeepCopy()
	owned.UID = "owned"
	owned.ResourceVersion = "2"
	if err = client.Tracker().Create(appsv1.SchemeGroupVersion.WithResource("deployments"), owned, "apps"); err != nil {
		t.Fatal(err)
	}
	request.Expected[0].UID, request.Expected[0].ResourceVersion = "owned", "1"
	if _, err = c.Apply(context.Background(), request); err == nil || writes(client) != 0 {
		t.Fatal("stale resourceVersion updated current workload", err)
	}
}

func TestWorkloadObserveRefusesSpecDriftAndNamespaceReplacementWithoutEffects(t *testing.T) {
	c, client, _, request := workloadFixture(t)
	if _, err := c.Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	deployment, err := client.AppsV1().Deployments("apps").Get(context.Background(), request.Expected[0].Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	deployment.Spec.Template.Spec.Containers[0].Image = "unreviewed-image"
	if _, err = client.AppsV1().Deployments("apps").Update(context.Background(), deployment, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	before := writes(client)
	if op, err := c.Observe(context.Background(), request.Identity); err != nil || op.State != "unknown" || writes(client) != before {
		t.Fatal("annotation alone hid live spec drift", err)
	}
	namespace, _ := client.CoreV1().Namespaces().Get(context.Background(), "apps", metav1.GetOptions{})
	namespace.UID = "replacement-namespace"
	if _, err = client.CoreV1().Namespaces().Update(context.Background(), namespace, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	before = writes(client)
	if _, err = c.Observe(context.Background(), request.Identity); err == nil || writes(client) != before {
		t.Fatal("namespace replacement was adopted", err)
	}
}

func TestWorkloadComparisonAcceptsDefaultEquivalentSpecAndRejectsUnplannedPlacementIdentityAndSecurity(t *testing.T) {
	_, _, _, request := workloadFixture(t)
	_, resources, err := prepare(request)
	if err != nil {
		t.Fatal(err)
	}
	expected := resources[0].deployment
	defaulted := expected.DeepCopy()
	pod := &defaulted.Spec.Template.Spec
	pod.RestartPolicy, pod.DNSPolicy, pod.SchedulerName = corev1.RestartPolicyAlways, corev1.DNSClusterFirst, corev1.DefaultSchedulerName
	pod.TerminationGracePeriodSeconds, pod.EnableServiceLinks = pointer(int64(30)), pointer(true)
	pod.PreemptionPolicy, pod.SecurityContext = pointer(corev1.PreemptLowerPriority), &corev1.PodSecurityContext{}
	pod.ServiceAccountName, pod.DeprecatedServiceAccount = "default", "default"
	pod.Containers[0].TerminationMessagePath, pod.Containers[0].TerminationMessagePolicy = "/dev/termination-log", corev1.TerminationMessageReadFile
	defaulted.Spec.RevisionHistoryLimit, defaulted.Spec.ProgressDeadlineSeconds = pointer(int32(10)), pointer(int32(600))
	defaulted.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType, RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: pointer(intstr.FromString("25%")), MaxUnavailable: pointer(intstr.FromString("25%"))}}
	// Controlled default-equivalent fixture, not an observed API server result.
	if !deploymentMatches(expected, defaulted) {
		t.Fatal("default-equivalent fields changed the intended workload")
	}
	for _, row := range []struct {
		name   string
		change func(*corev1.PodSpec)
	}{
		{"pod security context", func(p *corev1.PodSpec) { p.SecurityContext.RunAsUser = pointer(int64(1234)) }},
		{"service account", func(p *corev1.PodSpec) { p.ServiceAccountName = "privileged-account" }},
		{"node selector", func(p *corev1.PodSpec) { p.NodeSelector = map[string]string{"pool": "unreviewed"} }},
		{"affinity", func(p *corev1.PodSpec) { p.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{}} }},
		{"tolerations", func(p *corev1.PodSpec) {
			p.Tolerations = []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}}
		}},
		{"runtime", func(p *corev1.PodSpec) { p.RuntimeClassName = pointer("unreviewed-runtime") }},
		{"scheduler", func(p *corev1.PodSpec) { p.SchedulerName = "another-scheduler" }},
		{"DNS", func(p *corev1.PodSpec) { p.DNSPolicy = corev1.DNSNone }},
		{"host port", func(p *corev1.PodSpec) { p.Containers[0].Ports[0].HostPort = 8080 }},
	} {
		t.Run(row.name, func(t *testing.T) {
			changed := defaulted.DeepCopy()
			row.change(&changed.Spec.Template.Spec)
			if deploymentMatches(expected, changed) {
				t.Fatal("unreviewed pod field passed readiness comparison")
			}
		})
	}
	service := resources[1].service.DeepCopy()
	service.Spec.ClusterIP = "10.0.0.2"
	service.Spec.ClusterIPs = []string{"10.0.0.2"}
	service.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
	service.Spec.IPFamilyPolicy = pointer(corev1.IPFamilyPolicySingleStack)
	service.Spec.SessionAffinity, service.Spec.InternalTrafficPolicy = corev1.ServiceAffinityNone, pointer(corev1.ServiceInternalTrafficPolicyCluster)
	if !serviceMatches(resources[1].service, service) {
		t.Fatal("assigned internal service identity was rejected")
	}
	service.Spec.ExternalIPs = []string{"203.0.113.2"}
	if serviceMatches(resources[1].service, service) {
		t.Fatal("unplanned external exposure passed comparison")
	}
}
