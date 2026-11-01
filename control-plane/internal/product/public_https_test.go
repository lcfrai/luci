package product

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"strings"
	"testing"
	"time"
)

func publicHTTPSStoredFixture(t *testing.T) (*Server, project.Project, context.Context, storedPublicHTTPS) {
	t.Helper()
	s, p, _, ctx := azureBlueprintFixture(t)
	actor, e := management.CaptureDeploymentActor(ctx, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	s.mu.RLock()
	scope, e := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	key := "review_original_123456"
	id := publicHTTPSReviewID(actor, key)
	h := strings.Repeat("a", 64)
	row := storedPublicHTTPS{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Actor: actor, DNSCredential: PublicHTTPSDNSCredential{TenantID: "11111111-1111-1111-1111-111111111111", ClientID: "22222222-2222-2222-2222-222222222222", ClientSecret: "private-secret-never-public"}, SourceBlueprintDigest: h, SourceContextDigest: h, SourceRunDigest: h, SourceCheckDigest: h, ReviewRequestDigest: h, DesiredReplicas: 1, PublicHTTPSStatus: PublicHTTPSStatus{ID: id, ProjectID: p.ID, Revision: 1, ReviewKey: key, Actions: []PublicHTTPSActionReceipt{}, State: "reviewed", Resources: []PublicHTTPSResource{}, Message: "Reviewed only."}}
	row.Plan = PublicHTTPSPlan{ID: id, ProjectID: p.ID, DeploymentID: "workload-" + strings.Repeat("b", 32), Destination: PublicHTTPSDestination{TargetID: "kube-main", TargetFingerprint: h, Namespace: "apps", NamespaceUID: "namespace-original", ServiceID: "app", ServiceName: "project-app", ServiceUID: "app-original", ServicePort: 8080}, Hostname: "app.example.com", Path: "/", PathType: "Prefix", AzureDNSZoneID: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/dns/providers/Microsoft.Network/dnsZones/example.com", AzureDNSZoneETag: "zone-etag", AzureDNSRecordName: "app", AzureJobID: "job-original", AzureJobPlanDigest: h, AzureClusterUID: "cluster-original", TraefikImage: deploymentkube.PublicHTTPSImageDefault, Profile: deploymentkube.PublicHTTPSProfile, StorageClass: "managed-csi", StorageClassUID: "class-original", StorageClassVersion: "1", StorageBindingMode: "Immediate", StorageGiB: 1, ACMEEmail: "owner@example.com", DNSClientID: row.DNSCredential.ClientID, MonthlyBudgetUSD: 10, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano)}
	row.Plan.Digest = publicHTTPSPlanDigest(row.Plan)
	objects, e := deploymentkube.PublicHTTPSObjects(publicHTTPSSpec(row), publicHTTPSCredentialData(row))
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range objects {
		row.Resources = append(row.Resources, PublicHTTPSResource{Kind: o.Kind, Name: o.Name, State: "planned"})
	}
	if !validStoredPublicHTTPS(row, p) {
		t.Fatal("invalid fixture")
	}
	path, e := s.publicHTTPSPath(p, id)
	if e != nil || publicHTTPSWrite(path, row, 128<<10) != nil {
		t.Fatal(e)
	}
	return s, p, ctx, row
}
func TestPublicHTTPSPrivateOriginalReceiptReplaysWithoutEffectPermit(t *testing.T) {
	s, p, ctx, row := publicHTTPSStoredFixture(t)
	input := PublicHTTPSActionInput{ExpectedRevision: 1, RequestKey: "apply_original_123456", Action: "apply"}
	row.ApplyAttempted = true
	row.Revision = 2
	row.Actions = []PublicHTTPSActionReceipt{{RequestKey: input.RequestKey, RequestDigest: reviewDigest(input), Action: "apply", State: "unknown", StartedRevision: 1}}
	path, _ := s.publicHTTPSPath(p, row.ID)
	if publicHTTPSWrite(path, row, 128<<10) != nil {
		t.Fatal("write")
	}
	got, first, e := s.beginPublicHTTPSAction(ctx, p, row.Actor, row.ID, input)
	if e != nil || first || got.Revision != 2 || got.Actions[0].State != "unknown" {
		t.Fatal("unknown effect re-admitted", first, e)
	}
	input.ExpectedRevision = 2
	if _, _, e = s.beginPublicHTTPSAction(ctx, p, row.Actor, row.ID, input); e == nil {
		t.Fatal("same key changed payload")
	}
	raw, _ := json.Marshal(got.PublicHTTPSStatus)
	if strings.Contains(string(raw), "private-secret") || strings.Contains(string(raw), "dnsCredential") {
		t.Fatal("secret escaped")
	}
	row.Actions = append(row.Actions, row.Actions[0])
	if validStoredPublicHTTPS(row, p) {
		t.Fatal("duplicate action key")
	}
}
func TestPublicHTTPSStoppedPublicPolicyStillAllowsOriginalMetadataAuthority(t *testing.T) {
	s, _, ctx, row := publicHTTPSStoredFixture(t)
	original := s.coordinatorDeploymentAuthorize
	s.coordinatorDeploymentAuthorize = func(c context.Context, a management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		v, e := original(c, a, allocate)
		v.Policy.AllowCloudProvisioning = false
		v.Policy.AllowPublicPublishing = false
		return v, e
	}
	if s.publicHTTPSAuthority(ctx, row.Actor, false, 0) != nil || s.publicHTTPSAuthority(ctx, row.Actor, true, 10) == nil {
		t.Fatal("cleanup incorrectly needs new public allocation")
	}
	s.coordinatorDeploymentAuthorize = func(context.Context, management.ExecutionActorRef, bool) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if s.publicHTTPSAuthority(ctx, row.Actor, false, 0) == nil {
		t.Fatal("late revoked actor retained authority")
	}
}
func TestPublicHTTPSDNSRequiresPositiveOriginalAcknowledgementAndCasefoldsARM(t *testing.T) {
	_, _, _, row := publicHTTPSStoredFixture(t)
	row.Address = "8.8.8.8"
	row.DNSAttempted = true
	row.DNSAcknowledged = true
	row.DNSRecordETag = "original-etag"
	record := publicHTTPSDNSRecord{ID: strings.ToLower(row.Plan.AzureDNSZoneID) + "/a/app", Name: "app", Type: "Microsoft.Network/dnsZones/A", ETag: row.DNSRecordETag}
	record.Properties.TTL = 300
	record.Properties.ARecords = append(record.Properties.ARecords, struct {
		IP string `json:"ipv4Address"`
	}{IP: row.Address})
	record.Properties.Metadata = publicHTTPSDNSMetadata(row)
	if !publicHTTPSDNSMatches(row, record) {
		t.Fatal("canonical Azure casing")
	}
	row.DNSAcknowledged = false
	if publicHTTPSDNSMatches(row, record) {
		t.Fatal("matching labels adopted an unknown DNS write")
	}
	zone := publicHTTPSZone{ID: strings.ToLower(row.Plan.AzureDNSZoneID), Name: "example.com", ETag: row.Plan.AzureDNSZoneETag}
	zone.Properties.ZoneType = "Public"
	if !publicHTTPSZoneMatches(row, zone) {
		t.Fatal("reviewed zone seal")
	}
	zone.ETag = "recreated"
	if publicHTTPSZoneMatches(row, zone) {
		t.Fatal("changed parent zone admitted")
	}
	canonical := canonicalPublicHTTPSZoneID(strings.ToLower(row.Plan.AzureDNSZoneID))
	if canonical == "" || !strings.EqualFold(canonical, row.Plan.AzureDNSZoneID) {
		t.Fatal("UI lowercase zone rejected")
	}
	row.DNSAcknowledged = true
	record.ETag = "replacement"
	if publicHTTPSDNSMatches(row, record) {
		t.Fatal("replacement DNS ETag adopted")
	}
}
func TestPublicHTTPSListPreservesLiveEndpointsAtCapacityAndRepairsReplayIndex(t *testing.T) {
	s, p, _, row := publicHTTPSStoredFixture(t)
	path, index, e := s.readPublicHTTPSIndex(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 128; i++ {
		copy := row
		copy.ReviewKey = "review_original_" + strings.Repeat("a", i+1)
		if len(copy.ReviewKey) > 128 {
			copy.ReviewKey = "review_other_" + strings.Repeat("b", i-100)
		}
		copy.ID = publicHTTPSReviewID(copy.Actor, copy.ReviewKey)
		copy.Plan.ID = copy.ID
		copy.State = "paused"
		copy.ApplyAttempted = true
		copy.DesiredReplicas = 0
		copy.Plan.Digest = publicHTTPSPlanDigest(copy.Plan)
		name := deploymentkube.PublicHTTPSName(p.ID, copy.ID)
		for j := range copy.Resources {
			copy.Resources[j].Name = name
		}
		record, _ := s.publicHTTPSPath(p, copy.ID)
		if publicHTTPSWrite(record, copy, 128<<10) != nil {
			t.Fatal("write")
		}
		index.IDs = append(index.IDs, copy.ID)
	}
	if publicHTTPSWrite(path, index, 128<<10) != nil {
		t.Fatal("index")
	}
	s.publicHTTPSMu.Lock()
	e = s.publicHTTPSRememberLocked(p, row.ID)
	s.publicHTTPSMu.Unlock()
	if !errors.Is(e, ErrPublicHTTPSCapacity) {
		t.Fatal("live endpoint evicted", e)
	}
}

func TestPublicHTTPSOriginalStepContinuationNeverReplaysAttemptedUnknown(t *testing.T) {
	_, _, _, row := publicHTTPSStoredFixture(t)
	pvc := publicHTTPSResource(&row, "PersistentVolumeClaim")
	pvc.Attempted = true
	pvc.UID = "original-pvc"
	pvc.State = "observed"
	if create, e := publicHTTPSCreationStep(pvc); e != nil || create {
		t.Fatal("acknowledged phase must be verified, not recreated", e)
	}
	secret := publicHTTPSResource(&row, "Secret")
	if create, e := publicHTTPSCreationStep(secret); e != nil || !create || !publicHTTPSHasUnattempted(row) {
		t.Fatal("crash before next marker stranded original", e)
	}
	secret.Attempted = true
	secret.State = "unknown"
	if create, e := publicHTTPSCreationStep(secret); e == nil || create {
		t.Fatal("unknown attempted phase became a fresh write")
	}
	secret.UID = "original-secret"
	secret.DeleteAttempted = true
	if create, e := publicHTTPSCreationStep(secret); e == nil || create {
		t.Fatal("retirement restored creation")
	}
}

func TestPublicHTTPSRetainedAppDebuggingDoesNotRequireCurrentPublicReadiness(t *testing.T) {
	s, p, ctx, row := publicHTTPSStoredFixture(t)
	row.State = "retired"
	row.ApplyAttempted = true
	row.RetireAttempted = true
	row.DesiredReplicas = 0
	row.DNSCredential = PublicHTTPSDNSCredential{}
	path, _ := s.publicHTTPSPath(p, row.ID)
	if publicHTTPSWrite(path, row, 128<<10) != nil {
		t.Fatal("write")
	}
	d := row.Plan.Destination
	target := deploymentkube.Target{ID: d.TargetID, Fingerprint: d.TargetFingerprint, Namespace: d.Namespace, NamespaceUID: d.NamespaceUID}
	blueprint := deploymentplan.Blueprint{ProjectID: p.ID, Infrastructure: deploymentplan.Infrastructure{Namespace: d.Namespace}, Services: []deploymentplan.Service{{ID: d.ServiceID, Kind: "app", Port: d.ServicePort}}, PublicRoute: &deploymentplan.PublicRoute{Profile: deploymentplan.ManagedPublicRouteProfile, ManagedHTTPSID: row.ID, ServiceID: d.ServiceID, Hostname: row.Plan.Hostname, Path: row.Plan.Path, PathType: row.Plan.PathType}}
	if _, e := s.managedPublicHTTPSRow(ctx, row.Actor, blueprint, target, false); e != nil {
		t.Fatal("retired TLS blocked historical app metadata/debugging", e)
	}
	if _, e := s.managedPublicHTTPSRow(ctx, row.Actor, blueprint, target, true); e == nil {
		t.Fatal("retired endpoint authorized new delivery")
	}
	if publicHTTPSBindingSpec(row).Replicas != 1 {
		t.Fatal("pause rewrote the historical binding")
	}
}
func TestPublicHTTPSReceiptCapacityKeepsUnresolvedReadAndEffectKeys(t *testing.T) {
	s, p, _, stored := publicHTTPSStoredFixture(t)
	stored.Revision = 2
	stored.State = "unknown"
	stored.Actions = []PublicHTTPSActionReceipt{{RequestKey: "observe_original_123456", RequestDigest: strings.Repeat("a", 64), Action: "observe", State: "unknown", StartedRevision: 1}}
	path, _ := s.publicHTTPSPath(p, stored.ID)
	if publicHTTPSWrite(path, stored, 128<<10) != nil {
		t.Fatal("write")
	}
	settled, e := s.settlePublicHTTPSAction(p, stored, stored.Actions[0].RequestKey, true)
	if e != nil || settled.Actions[0].State != "settled" || settled.State != "unknown" || settled.HTTPSVerified || settled.ApplicationHealthVerified {
		t.Fatal("inspection receipt invented readiness", e)
	}
	row := storedPublicHTTPS{}
	for i := 0; i < 32; i++ {
		row.Actions = append(row.Actions, PublicHTTPSActionReceipt{RequestKey: strings.Repeat("a", i+1), Action: "observe", State: "unknown"})
	}
	if publicHTTPSTrimActions(&row) || len(row.Actions) != 32 {
		t.Fatal("unresolved inspection key evicted")
	}
	row.Actions[0].State = "settled"
	if !publicHTTPSTrimActions(&row) || len(row.Actions) != 31 || row.Actions[0].RequestKey != "aa" {
		t.Fatal("settled receipt did not roll conservatively")
	}
}
