package product

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"strings"
	"testing"
	"time"
)

func storageProductFixture(t *testing.T) (*Server, context.Context, CloudJob, storedRuntimePreparation, *int, *bool) {
	t.Helper()
	s, p, req, ctx := azureBlueprintFixture(t)
	tls := func(name string) deploymentplan.TelemetryTLSReference {
		return deploymentplan.TelemetryTLSReference{ResourceName: name, CAKey: "ca", CertKey: "cert", KeyKey: "key"}
	}
	b := req.Foundation.Blueprint
	for _, kind := range []string{"metrics-store", "trace-store"} {
		port, profile := 9090, deploymentplan.PrometheusStorageProfile
		if kind == "trace-store" {
			port, profile = 3200, deploymentplan.TempoStorageProfile
		}
		service := deploymentplan.Service{ID: kind, Kind: kind, Image: "registry.example/store@sha256:" + strings.Repeat("d", 64), Port: port, Replicas: 1, DeploymentStrategy: "recreate", Resources: deploymentplan.ResourceBudget{RequestMilliCPU: 250, LimitMilliCPU: 1000, RequestMemoryMiB: 512, LimitMemoryMiB: 1024}, TelemetryStorage: &deploymentplan.TelemetryStorage{Profile: profile, ClaimName: kind + "-data", CapacityGiB: 10, RetentionHours: 168, ServerTLS: tls(kind + "-server"), QueryTLS: tls(kind + "-query")}}
		if kind == "trace-store" {
			service.TelemetryStorage.Azure = &deploymentplan.TelemetryAzureBlocks{AccountName: "ownedtraces", ContainerName: "traces", AccountKey: deploymentplan.TelemetryAuthorizationReference{ResourceName: "trace-account", Key: "account.key"}}
		}
		b.Services = append(b.Services, service)
	}
	plan, err := deploymentplan.Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	req.Foundation = &deploymentplan.AzureFoundationRequest{Blueprint: b, ExpectedBlueprintDigest: plan.Digest}
	req.Domain, req.Size, req.SSHPublicKey = "", "", ""
	req.TelemetryStorage = &deploymentplan.AzureTelemetryStorageSpec{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", AccountName: "ownedtraces", ContainerName: "traces", RetentionDays: 7}
	if _, err = s.SaveDeploymentBlueprint(ctx, p.ID, DeploymentBlueprintSave{IdempotencyKey: "store-fixture-save", Blueprint: b}, allowBlueprintAccess); err != nil {
		t.Fatal(err)
	}
	keys := 0
	revoke := false
	var job CloudJob
	now := time.Now().UTC()
	accountID := "/subscriptions/" + req.TelemetryStorage.SubscriptionID + "/resourceGroups/" + req.Name + "/providers/Microsoft.Storage/storageAccounts/ownedtraces"
	s.runner = func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if revoke {
			return "", management.ErrExecutionAuthority
		}
		if exe != "az" {
			t.Fatal("unexpected executable")
		}
		if len(args) > 1 && args[0] == "account" && args[1] == "show" {
			return `{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","tenantId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","environmentName":"AzureCloud","state":"Enabled","user":{"name":"operator@example.com","type":"user"}}`, nil
		}
		if len(args) > 1 && args[0] == "account" && args[1] == "get-access-token" {
			token := azureClusterTokenFixture(time.Now(), func(c map[string]any) { c["aud"] = "https://management.azure.com/" })
			raw, _ := json.Marshal(map[string]string{"accessToken": token, "tokenType": "Bearer", "tenant": req.TelemetryStorage.TenantID, "subscription": req.TelemetryStorage.SubscriptionID})
			return string(raw), nil
		}
		if len(args) > 1 && args[0] == "resource" && args[1] == "show" {
			if strings.Contains(strings.Join(args, " "), "/containers/") {
				return `{"id":"` + accountID + `/blobServices/default/containers/traces","etag":"version-1","properties":{"publicAccess":"None"}}`, nil
			}
			raw, _ := json.Marshal(map[string]any{"id": accountID, "name": "ownedtraces", "type": "Microsoft.Storage/storageAccounts", "location": req.Region, "kind": "StorageV2", "sku": map[string]string{"name": "Standard_LRS"}, "tags": map[string]string{"managed-by": "luci", "luci-plan": job.PlanID}, "properties": map[string]any{"creationTime": now.Format(time.RFC3339Nano), "provisioningState": "Succeeded", "minimumTlsVersion": "TLS1_2", "supportsHttpsTrafficOnly": true, "allowBlobPublicAccess": false, "allowSharedKeyAccess": true}})
			return string(raw), nil
		}
		if len(args) > 3 && args[0] == "storage" && args[1] == "account" && args[2] == "keys" && args[3] == "list" {
			keys++
			value := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 64)))
			raw, _ := json.Marshal([]map[string]string{{"keyName": "key1", "permissions": "FULL", "value": value}, {"keyName": "key2", "permissions": "FULL", "value": value}})
			return string(raw), nil
		}
		return "", errors.New("unconfigured controlled provider response")
	}
	binding, err := s.prepareFoundationBinding(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	cloud, err := s.buildTelemetryStorageCloudPlan(req, *binding)
	if err != nil {
		t.Fatal(err)
	}
	cloud.Foundation = binding
	binding.PlanDigest = foundationPlanDigest(cloud)
	if !validFoundationPlan(cloud) {
		t.Fatal("invalid server storage plan")
	}
	job = CloudJob{ID: "cj-original-storage", Kind: "telemetry-storage", PlanID: cloud.ID, Provider: "azure", Name: req.Name, Region: req.Region, Plan: cloud, Owned: true, Status: "resources_ready", CreatedAt: now, UpdatedAt: now}
	observation, err := s.observeTelemetryStorage(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	job.TelemetryStorage = &observation
	s.jobs[job.ID] = job
	row := storedRuntimePreparation{Template: b, RuntimePreparation: RuntimePreparation{Plan: RuntimePreparationPlan{ID: "prep-" + strings.Repeat("a", 32), ProjectID: p.ID, Actor: binding.Actor, MonthlyBudgetUSD: 50}}}
	return s, ctx, job, row, &keys, &revoke
}
func TestOwnedBlobKeyImportRetainsOnlyOriginalPrivateSecretAndStopsAfterDeletion(t *testing.T) {
	s, ctx, job, row, calls, _ := storageProductFixture(t)
	binding, plan, private, err := s.reviewRuntimeBlobSecret(ctx, row.Plan.Actor, row, job.ID)
	if err != nil || *calls != 1 || binding == nil || plan == nil || private == nil || len(private.Data["account.key"]) == 0 {
		t.Fatal("private original import missing", err)
	}
	public, _ := json.Marshal(struct {
		Binding *RuntimeTelemetryBlobBinding
		Plan    *RuntimeSecretPlan
	}{binding, plan})
	for _, value := range private.Data {
		if strings.Contains(string(public), string(value)) {
			t.Fatal("credential escaped public metadata")
		}
	}
	row.Plan.Resources = &RuntimeResourcePlan{TelemetryBlob: binding, Secrets: []RuntimeSecretPlan{*plan}}
	if !validRuntimeBlobBinding(row) {
		t.Fatal("original private receipt not valid")
	}
	job.DeleteAttempted = true
	s.jobs[job.ID] = job
	if s.runtimeBlobCurrent(ctx, row, false) == nil {
		t.Fatal("deleting storage still admitted Secret effects")
	}
	if _, _, _, err = s.reviewRuntimeBlobSecret(ctx, row.Plan.Actor, row, job.ID); err == nil || *calls != 1 {
		t.Fatal("deleted/unknown source reread or adopted")
	}
	if !validRuntimeBlobBinding(row) {
		t.Fatal("deleting external Blob erased the independently retained resource ownership needed for cleanup")
	}
}
func TestOwnedBlobImportRejectsLostAccessAndUnknownOriginalWithoutNewKeyRead(t *testing.T) {
	s, ctx, job, row, calls, revoke := storageProductFixture(t)
	*revoke = true
	if binding, _, _, err := s.reviewRuntimeBlobSecret(ctx, row.Plan.Actor, row, job.ID); err == nil || binding != nil || *calls != 0 {
		t.Fatal("unavailable original yielded private credential")
	}
	*revoke = false
	job.Status = "unknown"
	s.jobs[job.ID] = job
	if _, _, _, err := s.reviewRuntimeBlobSecret(ctx, row.Plan.Actor, row, job.ID); err == nil || *calls != 0 {
		t.Fatal("unknown original acted as a new request")
	}
}
func TestOwnedBlobKeyReadLosingCurrentSecretsCannotPublishPrivatePreparation(t *testing.T) {
	s, ctx, job, row, calls, _ := storageProductFixture(t)
	baseAuthorize, baseRunner := s.coordinatorDeploymentAuthorize, s.runner
	revoked := false
	s.coordinatorDeploymentAuthorize = func(ctx context.Context, actor management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		if revoked {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return baseAuthorize(ctx, actor, allocate)
	}
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		raw, err := baseRunner(ctx, exe, args, dir)
		if len(args) > 3 && args[0] == "storage" && args[1] == "account" && args[2] == "keys" && args[3] == "list" {
			revoked = true
		}
		return raw, err
	}
	binding, public, private, err := s.reviewRuntimeBlobSecret(ctx, row.Plan.Actor, row, job.ID)
	if err == nil || binding != nil || public != nil || private != nil || *calls != 1 {
		t.Fatal("late grant loss retained/returned new credential material")
	}
}
func TestOwnedTelemetryGeneratedTLSCoversExactStoreAndNeverRetainsSigner(t *testing.T) {
	_, _, _, row, _, _ := storageProductFixture(t)
	plans, private, err := runtimeStorageTLSSecrets(row.Template)
	if err != nil || len(plans) != 4 || len(private) != 4 {
		t.Fatal("store server/query roles missing", err)
	}
	for i, p := range plans {
		bytes := private[i].Data["cert"]
		block, _ := pem.Decode(bytes)
		if block == nil {
			t.Fatal("no issued leaf")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || cert.IsCA {
			t.Fatal("signer leaked as leaf")
		}
		if p.Kind == "telemetry-server" {
			dns := deploymentplan.ResourceName(row.Template.ProjectID, p.ServiceID) + "." + row.Template.Infrastructure.Namespace + ".svc"
			if cert.VerifyHostname(dns) != nil {
				t.Fatal("derived store identity missing")
			}
		} else if len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
			t.Fatal("query pair is not a client")
		}
		if len(private[i].Data) != 3 {
			t.Fatal("unexpected signer/private value field")
		}
	}
}
func TestStorageARMAudienceIsClosedWithoutWideningExistingAKSToken(t *testing.T) {
	now := time.Now()
	principal := deploymentplan.AzureClusterPrincipal{SubscriptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Kind: "User", AccountName: "operator@example.com"}
	token := azureClusterTokenFixture(now, func(c map[string]any) { c["aud"] = "https://management.azure.com/" })
	if _, err := azureBearerClaimsForAudience(token, principal, now, "https://management.azure.com/"); err != nil {
		t.Fatal(err)
	}
	if _, err := azureClusterBearerClaims(token, principal, now); err == nil {
		t.Fatal("ARM token admitted as Kubernetes bearer")
	}
	if _, err := azureBearerClaimsForAudience(token, principal, now, "https://untrusted.example/"); err == nil {
		t.Fatal("arbitrary audience admitted")
	}
}
