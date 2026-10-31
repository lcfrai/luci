package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func aksProductFixture(t *testing.T) (*Server, context.Context, CloudJob, func(string, []string) (string, error)) {
	t.Helper()
	s, p, req, ctx := azureBlueprintFixture(t)
	req.Domain = ""
	req.SSHPublicKey = "ssh-ed25519 " + base64.StdEncoding.EncodeToString(make([]byte, 32))
	req.Cluster = &deploymentplan.AzureClusterSpec{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", KubernetesVersion: "1.34.1", NodeSKU: "Standard_D2s_v5", NodeCount: 1, AuthorizedAPICIDRs: []string{"8.8.8.8/32"}}
	if _, err := s.SaveDeploymentBlueprint(ctx, p.ID, DeploymentBlueprintSave{IdempotencyKey: "aks-product-fixture-save", Blueprint: req.Foundation.Blueprint}, allowBlueprintAccess); err != nil {
		t.Fatal(err)
	}
	var job CloudJob
	runner := func(exe string, args []string) (string, error) {
		if exe != "az" {
			t.Fatal("unexpected executable", exe)
		}
		if len(args) > 1 && args[0] == "account" && args[1] == "show" {
			return `{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","tenantId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","environmentName":"AzureCloud","state":"Enabled","user":{"name":"operator@example.com","type":"user"}}`, nil
		}
		if len(args) > 1 && args[0] == "account" && args[1] == "get-access-token" {
			raw, _ := json.Marshal(map[string]string{"accessToken": azureClusterTokenFixture(time.Now(), nil), "tokenType": "Bearer", "tenant": req.Cluster.TenantID, "subscription": req.Cluster.SubscriptionID})
			return string(raw), nil
		}
		if len(args) > 1 && args[0] == "resource" && args[1] == "show" {
			fit := job.Plan.Foundation.Cluster.Fit
			raw, _ := json.Marshal(map[string]any{"id": fit.ClusterResourceID, "name": fit.ClusterName, "type": "Microsoft.ContainerService/managedClusters", "location": job.Region, "tags": map[string]string{"luci-plan": job.PlanID}, "identity": map[string]string{"type": "SystemAssigned", "tenantId": fit.Spec.TenantID, "principalId": "dddddddd-dddd-dddd-dddd-dddddddddddd"}, "properties": map[string]any{"resourceUID": "cluster-original-generation", "provisioningState": "Succeeded", "kubernetesVersion": fit.Spec.KubernetesVersion, "currentKubernetesVersion": fit.Spec.KubernetesVersion, "nodeResourceGroup": fit.NodeResourceGroup, "fqdn": "owned.example", "enableRBAC": true, "disableLocalAccounts": true, "aadProfile": map[string]any{"managed": true, "enableAzureRBAC": true, "tenantID": fit.Spec.TenantID}, "apiServerAccessProfile": map[string]any{"enablePrivateCluster": false, "authorizedIPRanges": fit.Spec.AuthorizedAPICIDRs}, "agentPoolProfiles": []any{map[string]any{"name": "system", "vmSize": fit.Spec.NodeSKU, "count": fit.Spec.NodeCount, "mode": "System", "osType": "Linux", "osSKU": "Ubuntu", "enableAutoScaling": false, "enableNodePublicIP": false}}}})
			return string(raw), nil
		}
		if len(args) > 3 && args[0] == "rest" && args[2] == "GET" {
			return `{"value":[]}`, nil
		}
		return "", errors.New("unconfigured controlled provider response")
	}
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) { return runner(exe, args) }
	binding, err := s.prepareFoundationBinding(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.buildAzureClusterCloudPlan(req, *binding)
	if err != nil {
		t.Fatal(err)
	}
	plan.Foundation = binding
	binding.PlanDigest = foundationPlanDigest(plan)
	if !validFoundationPlan(plan) {
		t.Fatal("server-generated cluster binding invalid")
	}
	job = CloudJob{ID: "cj-aks-original", Kind: "aks", PlanID: plan.ID, Provider: "azure", Name: plan.Name, Region: plan.Region, Plan: plan, Owned: true, ResourceID: "/subscriptions/" + req.Cluster.SubscriptionID + "/resourceGroups/" + plan.Name, Status: "resources_ready", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	observed, err := s.observeAzureCluster(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	job.Cluster = &observed
	s.jobs[job.ID] = job
	return s, ctx, job, runner
}

func TestAzureClusterBootstrapUnknownGrantNeverReplaysAndPreservesOriginalPrincipal(t *testing.T) {
	s, ctx, job, base := aksProductFixture(t)
	actor, err := management.CaptureDeploymentActor(ctx, job.Plan.Foundation.Actor.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	row, err := s.ReviewAzureClusterCommission(ctx, actor, AzureClusterCommissionRequest{JobID: job.ID, Namespace: "project-apps", Mode: "create"})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := s.ReviewAzureClusterCommission(ctx, actor, AzureClusterCommissionRequest{JobID: job.ID, Namespace: "project-apps", Mode: "create"})
	if err != nil || duplicate.ID != row.ID || duplicate.Revision != row.Revision {
		t.Fatal("review did not return the canonical original after unlocking", err)
	}
	// Existing public API exposure does not require a NEW publishing grant to
	// commission a private namespace; current Deploy/Secrets/allocation still do.
	originalAuthorize := s.coordinatorDeploymentAuthorize
	s.coordinatorDeploymentAuthorize = func(c context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		value, err := originalAuthorize(c, ref, allocate)
		value.Policy.AllowPublicPublishing = false
		return value, err
	}
	puts := 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if len(args) > 3 && args[0] == "rest" && args[2] == "PUT" {
			puts++
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, row.RoleAssignmentID) || !strings.Contains(joined, row.Principal.ObjectID) || !strings.Contains(joined, deploymentplan.AzureAKSClusterAdminRole) {
				t.Fatal("scope, role or principal widened")
			}
			return "", errors.New("controlled response lost after the attempted boundary")
		}
		return base(exe, args)
	}
	if _, err = s.ApplyAzureClusterCommission(ctx, actor, row.ID, "access", true, true); err == nil {
		t.Fatal("lost response certified success")
	}
	retained, err := s.aksCommission(ctx, actor, row.ID)
	if err != nil || !retained.RoleAttempted || retained.State != "access-unknown" {
		t.Fatal("attempt-before-effect witness lost", err)
	}
	if _, err = s.ApplyAzureClusterCommission(ctx, actor, row.ID, "access", true, true); err != nil {
		t.Fatal("original observation failed", err)
	}
	if puts != 1 {
		t.Fatal("unknown role assignment replayed", puts)
	}
	if _, err = s.ApplyAzureClusterCommission(homeActor("bob"), actor, row.ID, "access", true, true); err == nil {
		t.Fatal("foreign request acquired original actor")
	}
}

func TestAzureClusterCommissionCASRejectsAttemptResetAndNamespaceReplacement(t *testing.T) {
	s, ctx, job, _ := aksProductFixture(t)
	actor := job.Plan.Foundation.Actor
	row, err := s.ReviewAzureClusterCommission(ctx, actor, AzureClusterCommissionRequest{JobID: job.ID, Namespace: "project-apps", Mode: "create"})
	if err != nil {
		t.Fatal(err)
	}
	row.NamespaceAttempted = true
	row.State = "namespace-unknown"
	if err = s.updateAKSCommission(ctx, actor, row, row.Revision); err != nil {
		t.Fatal(err)
	}
	row.Revision++
	reset := row
	reset.NamespaceAttempted = false
	if s.updateAKSCommission(ctx, actor, reset, row.Revision) == nil {
		t.Fatal("attempt marker cleared")
	}
	foreign := row
	foreign.Principal.ObjectID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	if s.updateAKSCommission(ctx, actor, foreign, row.Revision) == nil {
		t.Fatal("captured principal replaced")
	}
	object := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: row.Namespace, UID: types.UID("original-ns"), ResourceVersion: "1", Labels: map[string]string{"app.kubernetes.io/managed-by": "luci-cluster-commission", "luci.ai/commission": row.ID, "luci.ai/project": reviewDigest(row.ProjectID)[:12]}, Annotations: map[string]string{"luci.ai/cluster-resource-uid": row.Cluster.ResourceUID}}}
	if !namespaceForCommission(object, row) {
		t.Fatal("original owned namespace refused")
	}
	row.NamespaceUID = "original-ns"
	object.UID = "replacement-ns"
	if namespaceForCommission(object, row) {
		t.Fatal("replacement namespace adopted")
	}
}

func TestAzureClusterCurrentIdentityRefusalPrecedesAnyRoleMutation(t *testing.T) {
	s, ctx, job, base := aksProductFixture(t)
	actor := job.Plan.Foundation.Actor
	row, err := s.ReviewAzureClusterCommission(ctx, actor, AzureClusterCommissionRequest{JobID: job.ID, Namespace: "project-apps", Mode: "create"})
	if err != nil {
		t.Fatal(err)
	}
	puts := 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if len(args) > 1 && args[0] == "account" && args[1] == "show" {
			return `{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","tenantId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","environmentName":"AzureCloud","state":"Enabled","user":{"name":"different@example.com","type":"user"}}`, nil
		}
		if len(args) > 2 && args[0] == "rest" && args[2] == "PUT" {
			puts++
		}
		return base(exe, args)
	}
	if _, err = s.ApplyAzureClusterCommission(ctx, actor, row.ID, "access", true, true); err == nil || puts != 0 {
		t.Fatal("changed current account reached IAM", err, puts)
	}
}

func TestAzureClusterCredentialSourceNeverSerializesBearerOrDowngradesToStatic(t *testing.T) {
	s, ctx, job, _ := aksProductFixture(t)
	row := privateKubernetesTarget{KubernetesTargetDescriptor: KubernetesTargetDescriptor{ID: "cluster-" + strings.Repeat("a", 32), ProjectID: job.Plan.Foundation.Actor.ProjectID, Name: "Owned namespace", Server: "https://owned.example", Namespace: "project-apps", NamespaceUID: "ns-original", Revision: 1, CredentialKind: "azure-aks-entra"}, Credentials: KubernetesTargetCredentials{CA: kubeTestCA(t)}, AzureCluster: &AzureClusterCredentialSource{JobID: job.ID, PlanDigest: job.Plan.Foundation.PlanDigest, Cluster: *job.Cluster, Principal: job.Plan.Foundation.Cluster.Fit.Principal}}
	if validateKubernetesTargetCredentials(row) != nil {
		t.Fatal("closed dynamic source refused")
	}
	raw, _ := json.Marshal(row)
	if strings.Contains(string(raw), "accessToken") || strings.Contains(string(raw), `"token"`) {
		t.Fatal("bearer persisted")
	}
	copy := row
	copy.Credentials.Token = "must-not-persist"
	if validateKubernetesTargetCredentials(copy) == nil {
		t.Fatal("static token mixed into dynamic source")
	}
	cacheCtx := WithKubernetesTargetResolution(ctx)
	if cacheCtx == ctx || WithKubernetesTargetResolution(cacheCtx) != cacheCtx {
		t.Fatal("request cache unexpectedly global/replaced")
	}
	s.runner = func(context.Context, string, []string, string) (string, error) {
		t.Fatal("forged identity reached provider")
		return "", nil
	}
	if _, err := s.resolvedKubernetesClient(cacheCtx, deploymentkube.Identity{ProjectID: row.ProjectID, ActorID: "bob", TargetID: row.ID, TargetFingerprint: row.Fingerprint}, row); err == nil {
		t.Fatal("caller identity replaced")
	}
}
func TestAzureClusterDeleteAttemptPersistsBeforeProviderAndCannotRestoreCommissioning(t *testing.T) {
	s, ctx, job, base := aksProductFixture(t)
	deletes := 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if len(args) > 1 && args[0] == "group" && args[1] == "show" {
			raw, _ := json.Marshal(map[string]any{"id": job.ResourceID, "tags": map[string]string{"luci-job": job.ID}})
			return string(raw), nil
		}
		if len(args) > 1 && args[0] == "group" && args[1] == "delete" {
			deletes++
			if !s.jobsSnapshot(job.ID).DeleteAttempted {
				t.Fatal("delete reached provider before durable attempt")
			}
			return "", errors.New("controlled lost delete response")
		}
		return base(exe, args)
	}
	call := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/setup/cloud/jobs/"+job.ID+"/delete", strings.NewReader(`{"confirmName":"`+job.Name+`"}`)).WithContext(ctx)
		r.SetPathValue("jobId", job.ID)
		w := httptest.NewRecorder()
		s.deleteCloudJob(w, r)
		return w
	}
	if w := call(); w.Code != 502 || deletes != 1 {
		t.Fatal("unknown delete was not retained", w.Code, deletes)
	}
	s.updateJob(job, "resources_ready", "stale refresh")
	current := s.jobsSnapshot(job.ID)
	if !current.DeleteAttempted || current.Status != "deleting" {
		t.Fatal("old progress reset the delete attempt")
	}
	if w := call(); w.Code != 409 || deletes != 1 {
		t.Fatal("unknown deletion replayed", w.Code, deletes)
	}
	if _, err := s.ReviewAzureClusterCommission(ctx, job.Plan.Foundation.Actor, AzureClusterCommissionRequest{JobID: job.ID, Namespace: "project-apps", Mode: "create"}); err == nil {
		t.Fatal("deleting cluster offered new namespace commissioning")
	}
}
func TestAzureClusterAccessPolicyLossDuringFinalTokenReadCannotReachIAM(t *testing.T) {
	s, ctx, job, base := aksProductFixture(t)
	actor := job.Plan.Foundation.Actor
	row, err := s.ReviewAzureClusterCommission(ctx, actor, AzureClusterCommissionRequest{JobID: job.ID, Namespace: "project-apps", Mode: "create"})
	if err != nil {
		t.Fatal(err)
	}
	deny := false
	original := s.coordinatorDeploymentAuthorize
	s.coordinatorDeploymentAuthorize = func(c context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		value, err := original(c, ref, allocate)
		value.Policy.AllowCloudProvisioning = !deny
		return value, err
	}
	tokensAfterIntent, puts := 0, 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if len(args) > 1 && args[0] == "account" && args[1] == "get-access-token" {
			retained, err := s.aksCommission(ctx, actor, row.ID)
			if err != nil {
				t.Fatal(err)
			}
			if retained.RoleAttempted {
				tokensAfterIntent++
				if tokensAfterIntent == 3 {
					deny = true
				}
			}
		}
		if len(args) > 2 && args[0] == "rest" && args[2] == "PUT" {
			puts++
		}
		return base(exe, args)
	}
	if _, err = s.ApplyAzureClusterCommission(ctx, actor, row.ID, "access", true, true); err == nil || !deny || puts != 0 {
		t.Fatal("late allocation policy loss reached IAM", err, tokensAfterIntent, puts)
	}
	retained, err := s.aksCommission(ctx, actor, row.ID)
	if err != nil || !retained.RoleAttempted {
		t.Fatal("original attempted marker was discarded after refusal", err)
	}
}
func TestAzureClusterUnknownNamespaceNeverSealsFromLabelsButPermitsExplicitReviewedImport(t *testing.T) {
	s, ctx, job, base := aksProductFixture(t)
	actor := job.Plan.Foundation.Actor
	row, err := s.ReviewAzureClusterCommission(ctx, actor, AzureClusterCommissionRequest{JobID: job.ID, Namespace: "project-apps", Mode: "create"})
	if err != nil {
		t.Fatal(err)
	}
	row.RoleAttempted = true
	row.RoleObserved = true
	row.NamespaceAttempted = true
	row.State = "namespace-unknown"
	if err = s.updateAKSCommission(ctx, actor, row, row.Revision); err != nil {
		t.Fatal(err)
	}
	reads := 0
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if len(args) > 2 && args[0] == "rest" && args[2] == "GET" {
			assignment := azureClusterRoleAssignment{ID: row.RoleAssignmentID}
			assignment.Properties.PrincipalID = row.Principal.ObjectID
			assignment.Properties.PrincipalType = row.Principal.Kind
			assignment.Properties.RoleDefinitionID = row.RoleDefinitionID
			assignment.Properties.Scope = row.Cluster.ResourceID
			assignment.Properties.Description = "Luci reviewed bootstrap " + row.ID
			raw, _ := json.Marshal(map[string]any{"value": []azureClusterRoleAssignment{assignment}})
			return string(raw), nil
		}
		if len(args) > 2 && args[0] == "rest" && args[2] == "POST" {
			reads++
			t.Fatal("unknown original UID attempted namespace credential/client sealing")
		}
		return base(exe, args)
	}
	observed, err := s.ObserveAzureClusterCommission(ctx, actor, row.ID)
	if err != nil || observed.State != "namespace-unknown" || observed.NamespaceUID != "" || reads != 0 {
		t.Fatal("labels promoted an unknown Create", observed, err)
	}
	if !aksCommissionReviewBlocked(observed, "create", true) || !aksCommissionReviewBlocked(observed, "existing", false) || aksCommissionReviewBlocked(observed, "existing", true) {
		t.Fatal("unknown create replayed or explicit current-access import made permanently impossible")
	}
	// A known successful Create identity is mandatory on all subsequent reads.
	known := observed
	known.NamespaceUID = "original-returned-uid"
	object := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: known.Namespace, UID: "replacement-uid", ResourceVersion: "9", Labels: map[string]string{"app.kubernetes.io/managed-by": "luci-cluster-commission", "luci.ai/commission": known.ID, "luci.ai/project": reviewDigest(known.ProjectID)[:12]}, Annotations: map[string]string{"luci.ai/cluster-resource-uid": known.Cluster.ResourceUID}}}
	if namespaceForCommission(object, known) {
		t.Fatal("copied labels replaced a positively returned original UID")
	}
}
