package deploymentkube

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"strings"
	"testing"
)

func runtimeReferenceFixture(t *testing.T) (*Connector, *fake.Clientset, *memoryJournal, Request) {
	t.Helper()
	c, client, journal, request := workloadFixture(t)
	request.Blueprint.Services[0].Environment = []deploymentplan.EnvironmentReference{{Name: "ACCESS_TOKEN", Kind: "secret", ResourceName: "app-access", Key: "token"}}
	plan, err := deploymentplan.Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request.Identity.PlanDigest = plan.Digest
	if err = client.Tracker().Add(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "app-access", Namespace: "apps", UID: "secret-one", ResourceVersion: "7"}, Data: map[string][]byte{"token": []byte("SENTINEL-SECRET-VALUE")}}); err != nil {
		t.Fatal(err)
	}
	target, err := c.target(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.References, err = ReadReferences(context.Background(), target, request.Blueprint, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return c, client, journal, request
}

func TestRuntimeReferencesSealIdentityAndSelectedKeysWithoutValues(t *testing.T) {
	c, client, _, request := runtimeReferenceFixture(t)
	raw, _ := json.Marshal(request.References)
	if strings.Contains(string(raw), "SENTINEL") || len(request.References) != 1 || request.References[0].Keys[0] != "token" || request.References[0].UID != "secret-one" {
		t.Fatal("review leaked values or lost exact key identity")
	}
	if _, err := c.Apply(context.Background(), request); err != nil || writes(client) != 2 {
		t.Fatal("valid exact refs did not reach normal existing apply", err)
	}
}

func TestRuntimeReferenceRotationDuringAttemptFenceNeverWritesOrReplays(t *testing.T) {
	c, client, journal, request := runtimeReferenceFixture(t)
	journal.afterSave = func() {
		journal.afterSave = nil
		object, err := client.CoreV1().Secrets("apps").Get(context.Background(), "app-access", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		object.ResourceVersion = "8"
		if err = client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), object, "apps"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Apply(context.Background(), request); err == nil {
		t.Fatal("rotated reference reached a write")
	}
	if journal.operation == nil || !journal.operation.Resources[0].Attempted || writes(client) != 0 {
		t.Fatal("original attempt not retained or provider mutated")
	}
	observed, err := c.Observe(context.Background(), request.Identity)
	if err != nil || observed.State != "unknown" || observed.ReferenceState != "changed" {
		t.Fatalf("reference drift not explicit: %+v %v", observed, err)
	}
	_, _ = c.Apply(context.Background(), request)
	if writes(client) != 0 {
		t.Fatal("unknown attempt replayed")
	}
}

func TestRuntimeReferenceAuthorityRevocationWithholdsKeysAndMutation(t *testing.T) {
	c, client, _, request := runtimeReferenceFixture(t)
	allowed := true
	c.authorize = func(context.Context, Request, bool) error {
		if !allowed {
			return ErrRefused
		}
		return nil
	}
	client.PrependReactor("get", "secrets", func(ktesting.Action) (bool, runtime.Object, error) { allowed = false; return false, nil, nil })
	if op, err := c.Apply(context.Background(), request); !errors.Is(err, ErrRefused) || op.Request.Identity.ProjectID != "" || writes(client) != 0 {
		t.Fatal("late revoked authority leaked or mutated", err)
	}
}

func TestRuntimePVCModesEnforceDistinctSinglePodConsumers(t *testing.T) {
	row := ReferenceSnapshot{Phase: "Bound", VolumeMode: "Filesystem", VolumeName: "existing-volume", AccessModes: []string{"ReadWriteOncePod"}}
	one := deploymentplan.Service{ID: "one", Replicas: 1, DeploymentStrategy: "recreate"}
	two := one
	two.ID = "two"
	if !claimCompatible(row, []deploymentplan.Service{one}) || claimCompatible(row, []deploymentplan.Service{one, two}) {
		t.Fatal("RWOP single-pod identity was not enforced across services")
	}
	row.AccessModes = []string{"ReadWriteOnce"}
	if !claimCompatible(row, []deploymentplan.Service{one}) || claimCompatible(row, []deploymentplan.Service{one, two}) {
		t.Fatal("RWO needs one reviewed service when no co-location capability exists")
	}
	one.Replicas = 2
	if claimCompatible(row, []deploymentplan.Service{one}) {
		t.Fatal("unreviewed rollout overlap allowed")
	}
	row.AccessModes = []string{"ReadOnlyMany"}
	if !claimCompatible(row, []deploymentplan.Service{one}) {
		t.Fatal("shared read-only storage refused")
	}
	row.Phase = "Pending"
	if claimCompatible(row, []deploymentplan.Service{one}) {
		t.Fatal("pending storage certified")
	}
}
