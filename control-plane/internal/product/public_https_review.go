package product

import (
	"context"
	"errors"
	"math"
	"net/mail"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type publicHTTPSRPCFunc func(context.Context, deploymentkube.Target, string, string, string, any, any) error

var publicHTTPSPathGrammar = regexp.MustCompile(`^/[A-Za-z0-9/._~-]*$`)

func publicHTTPSCredentialData(row storedPublicHTTPS) map[string][]byte {
	return map[string][]byte{"tenant-id": []byte(row.DNSCredential.TenantID), "client-id": []byte(row.DNSCredential.ClientID), "client-secret": []byte(row.DNSCredential.ClientSecret)}
}
func publicHTTPSSpec(row storedPublicHTTPS) deploymentkube.PublicHTTPSResourceSpec {
	parts := publicHTTPSZoneID.FindStringSubmatch(row.Plan.AzureDNSZoneID)
	sub, group, zone := "", "", ""
	if len(parts) == 4 {
		sub, group, zone = parts[1], parts[2], parts[3]
	}
	d := row.Plan.Destination
	return deploymentkube.PublicHTTPSResourceSpec{ID: row.ID, ProjectID: row.ProjectID, Namespace: d.Namespace, ServiceName: d.ServiceName, ServiceUID: d.ServiceUID, ServicePort: d.ServicePort, Hostname: row.Plan.Hostname, Path: row.Plan.Path, PathType: row.Plan.PathType, Image: row.Plan.TraefikImage, StorageClass: row.Plan.StorageClass, Email: row.Plan.ACMEEmail, SubscriptionID: sub, ResourceGroup: group, ZoneName: zone, Replicas: row.DesiredReplicas}
}
func publicHTTPSBindingSpec(row storedPublicHTTPS) deploymentkube.PublicHTTPSResourceSpec {
	spec := publicHTTPSSpec(row)
	spec.Replicas = 1
	return spec
}
func publicHTTPSResource(row *storedPublicHTTPS, kind string) *PublicHTTPSResource {
	for i := range row.Resources {
		if row.Resources[i].Kind == kind {
			return &row.Resources[i]
		}
	}
	return nil
}
func publicHTTPSUnready(row *storedPublicHTTPS, state, message string) {
	row.State = state
	row.Message = message
	row.HTTPSVerified = false
	row.URL = ""
	row.HTTPStatus = 0
	row.ApplicationHealthVerified = false
}
func (s *Server) publicHTTPSDeployment(ctx context.Context, actor management.ExecutionActorRef, id string) (KubernetesReview, deploymentkube.Operation, error) {
	if !validKubeReceipt(id) {
		return KubernetesReview{}, deploymentkube.Operation{}, ErrPublicHTTPS
	}
	_, ledger, e := s.readKubernetesReviews(actor.ProjectID)
	if e != nil {
		return KubernetesReview{}, deploymentkube.Operation{}, e
	}
	for _, r := range ledger.Reviews {
		if r.ID == id && kubeActor(r.Request.Identity) == actor && r.Applied {
			j := kubernetesOperationJournal{server: s}
			op, e := j.Load(ctx, r.Request.Identity)
			if e != nil || op.RequestDigest != reviewDigest(r.Request) {
				return r, op, ErrPublicHTTPS
			}
			return r, op, nil
		}
	}
	return KubernetesReview{}, deploymentkube.Operation{}, ErrPublicHTTPS
}
func (s *Server) publicHTTPSAuthority(ctx context.Context, actor management.ExecutionActorRef, public bool, budget float64) error {
	if e := s.kubernetesAuthority(ctx, actor, public, true, budget); e != nil {
		return e
	}
	if public {
		return s.kubernetesPublicAuthority(ctx, actor)
	}
	return ctx.Err()
}
func (s *Server) publicHTTPSCurrentBlueprint(p project.Project) (deploymentplan.Blueprint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ledger, e := s.readDeploymentBlueprintLedger(p)
	if e != nil || len(ledger.Revisions) == 0 {
		return deploymentplan.Blueprint{}, ErrPublicHTTPS
	}
	return ledger.Revisions[len(ledger.Revisions)-1].Blueprint, nil
}
func publicHTTPSTargetIdentity(row storedPublicHTTPS) deploymentkube.Identity {
	d := row.Plan.Destination
	return deploymentkube.Identity{ProjectID: row.ProjectID, ActorID: row.Actor.UserID, TeamID: row.Actor.TeamID, TargetID: d.TargetID, TargetFingerprint: d.TargetFingerprint, OperationID: row.ID, PlanDigest: row.Plan.Digest}
}
func publicHTTPSTargetCacheKey(row storedPublicHTTPS) string {
	return reviewDigest(struct {
		Actor                               management.ExecutionActorRef
		TargetID, Fingerprint, NamespaceUID string
	}{row.Actor, row.Plan.Destination.TargetID, row.Plan.Destination.TargetFingerprint, row.Plan.Destination.NamespaceUID})
}

// The client is captured only within ONE request/bounded coordinator pass.
// Each use repeats current private descriptor/job/grants and namespace UID.
// Every mutation separately invokes the full existing Azure target resolver.
func (s *Server) publicHTTPSTarget(ctx context.Context, row storedPublicHTTPS) (deploymentkube.Target, error) {
	if s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		return deploymentkube.Target{}, management.ErrExecutionAuthority
	}
	_, ledger, e := s.readKubernetesTargets(row.ProjectID)
	if e != nil {
		return deploymentkube.Target{}, e
	}
	d := row.Plan.Destination
	matched := false
	for _, t := range ledger.Targets {
		if t.ID == d.TargetID && t.Fingerprint == d.TargetFingerprint && t.Namespace == d.Namespace && t.NamespaceUID == d.NamespaceUID && t.AzureCluster != nil && t.AzureCluster.JobID == row.Plan.AzureJobID && t.AzureCluster.PlanDigest == row.Plan.AzureJobPlanDigest && t.AzureCluster.Cluster.ResourceUID == row.Plan.AzureClusterUID {
			matched = true
		}
	}
	if !matched {
		return deploymentkube.Target{}, ErrPublicHTTPS
	}
	if _, e = s.publicHTTPSJob(row); e != nil {
		return deploymentkube.Target{}, e
	}
	cache, _ := ctx.Value(publicHTTPSBearerKey{}).(*publicHTTPSBearerCache)
	key := publicHTTPSTargetCacheKey(row)
	target := deploymentkube.Target{}
	if cache != nil {
		target = cache.targets[key]
	}
	if target.Client == nil {
		target, e = s.resolveKubernetesTarget(ctx, publicHTTPSTargetIdentity(row))
		if e != nil {
			return target, e
		}
		if cache != nil {
			cache.targets[key] = target
		}
	}
	if target.Namespace != d.Namespace || target.NamespaceUID != d.NamespaceUID || deploymentkube.PublicHTTPSNamespace(ctx, target) != nil || s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		return deploymentkube.Target{}, ErrPublicHTTPS
	}
	return target, nil
}
func (s *Server) publicHTTPSMutationTarget(ctx context.Context, row storedPublicHTTPS) (deploymentkube.Target, error) {
	cache, _ := ctx.Value(publicHTTPSBearerKey{}).(*publicHTTPSBearerCache)
	if cache != nil {
		delete(cache.targets, publicHTTPSTargetCacheKey(row))
	}
	// Full resolver repeats original CLI identity, cluster resource UID and grant
	// observations even when its own bounded credential cache reuses a bearer.
	return s.publicHTTPSTarget(ctx, row)
}

// New effects require current public/provisioning/Secrets+Deploy policy. Pause,
// retirement and original observation need current metadata/Secrets+Deploy but
// deliberately do not require source freshness, renewed spending or public grant.
func (s *Server) publicHTTPSGuard(ctx context.Context, row storedPublicHTTPS, public, source bool) error {
	if s.publicHTTPSAuthority(ctx, row.Actor, public, row.Plan.MonthlyBudgetUSD) != nil {
		return management.ErrExecutionAuthority
	}
	p, e := s.resolveProject(row.ProjectID)
	if e != nil {
		return e
	}
	_, latest, e := s.readPublicHTTPS(p, row.ID)
	if e != nil || latest.Actor != row.Actor || latest.Plan.Digest != row.Plan.Digest || public && (latest.RetireAttempted || latest.DesiredReplicas == 0) {
		return ErrPublicHTTPS
	}
	if _, e = s.publicHTTPSTarget(ctx, row); e != nil {
		return e
	}
	if (public || source) && s.publicHTTPSDestinationCurrent(ctx, p, row) != nil {
		return ErrPublicHTTPS
	}
	if source {
		review, _, e := s.publicHTTPSDeployment(ctx, row.Actor, row.Plan.DeploymentID)
		if e != nil || review.BlueprintDigest != row.SourceBlueprintDigest || review.ContextDigest != row.SourceContextDigest || review.RunInputDigest != row.SourceRunDigest || review.CheckDigest != row.SourceCheckDigest {
			return ErrPublicHTTPS
		}
		input := KubernetesReviewInput{TargetID: review.Target.ID, BlueprintRevision: review.BlueprintRevision, BlueprintDigest: review.BlueprintDigest, RunID: review.RunID, CheckID: review.CheckID, MonthlyBudgetUSD: row.Plan.MonthlyBudgetUSD}
		saved, currentContext, currentRun, currentCheck, se := s.kubernetesSource(ctx, row.Actor, input)
		if se != nil || saved.Digest != row.SourceBlueprintDigest || saved.PlanDigest != review.Request.Identity.PlanDigest || currentContext != row.SourceContextDigest || currentRun != row.SourceRunDigest || currentCheck != row.SourceCheckDigest {
			return ErrPublicHTTPS
		}
	}
	if _, e = s.publicHTTPSJob(row); e != nil {
		return e
	}
	return s.publicHTTPSAuthority(ctx, row.Actor, public, row.Plan.MonthlyBudgetUSD)
}
func (s *Server) ReviewPublicHTTPS(ctx context.Context, actor management.ExecutionActorRef, input PublicHTTPSReviewInput) (PublicHTTPSStatus, error) {
	ctx, cancel := context.WithTimeout(publicHTTPSRequestContext(ctx), 90*time.Second)
	defer cancel()
	input.AzureDNSZoneID = canonicalPublicHTTPSZoneID(input.AzureDNSZoneID)
	input.DNSCredential.TenantID = strings.ToLower(input.DNSCredential.TenantID)
	input.DNSCredential.ClientID = strings.ToLower(input.DNSCredential.ClientID)
	if !publicHTTPSKey.MatchString(input.ReviewKey) || !input.AcknowledgePublicCostsAndRenewal || !input.AcknowledgeACMETerms || !input.AcknowledgePrivateDNSCredential || input.MonthlyBudgetUSD <= 0 || math.IsNaN(input.MonthlyBudgetUSD) || math.IsInf(input.MonthlyBudgetUSD, 0) {
		return PublicHTTPSStatus{}, ErrPublicHTTPSInput
	}
	if s.publicHTTPSAuthority(ctx, actor, true, input.MonthlyBudgetUSD) != nil {
		return PublicHTTPSStatus{}, management.ErrExecutionAuthority
	}
	p, e := s.resolveProject(actor.ProjectID)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	id := publicHTTPSReviewID(actor, input.ReviewKey)
	requestDigest := reviewDigest(input)
	if _, old, e := s.readPublicHTTPS(p, id); e == nil {
		if old.Actor != actor || old.ReviewRequestDigest != requestDigest {
			return PublicHTTPSStatus{}, ErrPublicHTTPS
		}
		s.publicHTTPSMu.Lock()
		e = s.publicHTTPSRememberLocked(p, id)
		s.publicHTTPSMu.Unlock()
		if e != nil {
			return PublicHTTPSStatus{}, e
		}
		return old.PublicHTTPSStatus, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return PublicHTTPSStatus{}, e
	}
	email, e := mail.ParseAddress(input.ACMEEmail)
	zone := publicHTTPSZoneID.FindStringSubmatch(input.AzureDNSZoneID)
	if e != nil || email.Name != "" || email.Address != input.ACMEEmail || len(input.ACMEEmail) > 254 || !publicHTTPSPathGrammar.MatchString(input.Path) || path.Clean(input.Path) != input.Path || len(input.Path) > 256 || (input.PathType != "Exact" && input.PathType != "Prefix") || !azureSetupUUID.MatchString(input.DNSCredential.TenantID) || !azureSetupUUID.MatchString(input.DNSCredential.ClientID) || len(input.DNSCredential.ClientSecret) < 16 || len(input.DNSCredential.ClientSecret) > 4096 || strings.ContainsAny(input.DNSCredential.ClientSecret, "\x00\r\n") {
		return PublicHTTPSStatus{}, ErrPublicHTTPSInput
	}
	if len(zone) != 4 || !deploymentplan.ValidPublicHostname(input.Hostname) {
		return PublicHTTPSStatus{}, ErrPublicHTTPSDomain
	}
	relative := "@"
	if input.Hostname != zone[3] {
		if !strings.HasSuffix(input.Hostname, "."+zone[3]) {
			return PublicHTTPSStatus{}, ErrPublicHTTPSDomain
		}
		relative = strings.TrimSuffix(input.Hostname, "."+zone[3])
	}
	review, operation, e := s.publicHTTPSDeployment(ctx, actor, input.DeploymentID)
	if e != nil || s.authorizeKubernetesReview(ctx, review, false, true) != nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	var service *deploymentplan.Service
	for i := range review.Request.Blueprint.Services {
		v := &review.Request.Blueprint.Services[i]
		if v.ID == input.ServiceID && v.Kind == "app" {
			service = v
		}
	}
	if service == nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPSApplication
	}
	serviceName := deploymentplan.ResourceName(actor.ProjectID, service.ID)
	uid := ""
	for _, r := range operation.Resources {
		if r.Kind == "Service" && r.Name == serviceName && r.Attempted && r.ObservedUID != "" {
			uid = r.ObservedUID
		}
	}
	if uid == "" {
		return PublicHTTPSStatus{}, ErrPublicHTTPSApplication
	}
	_, targets, e := s.readKubernetesTargets(actor.ProjectID)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	var selected *privateKubernetesTarget
	for i := range targets.Targets {
		t := &targets.Targets[i]
		if t.ID == review.Target.ID && t.Fingerprint == review.Target.Fingerprint && t.AzureCluster != nil {
			selected = t
		}
	}
	if selected == nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	s.mu.RLock()
	job, ok := s.jobs[selected.AzureCluster.JobID]
	scope, se := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if !ok || se != nil || job.Cluster == nil || job.Plan.Foundation == nil || job.Plan.Foundation.Cluster == nil || job.Plan.Foundation.Actor != actor || job.Plan.Foundation.PlanDigest != selected.AzureCluster.PlanDigest || !strings.EqualFold(zone[1], job.Plan.Foundation.Cluster.Fit.Spec.SubscriptionID) || !strings.EqualFold(input.DNSCredential.TenantID, job.Plan.Foundation.Cluster.Fit.Spec.TenantID) {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	image := input.TraefikImage
	if image == "" {
		image = deploymentkube.PublicHTTPSImageDefault
	}
	now := time.Now().UTC()
	row := storedPublicHTTPS{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Actor: actor, DNSCredential: input.DNSCredential, ReviewRequestDigest: requestDigest, SourceBlueprintDigest: review.BlueprintDigest, SourceContextDigest: review.ContextDigest, SourceRunDigest: review.RunInputDigest, SourceCheckDigest: review.CheckDigest, DesiredReplicas: 1, PublicHTTPSStatus: PublicHTTPSStatus{ID: id, ProjectID: p.ID, Revision: 1, ReviewKey: input.ReviewKey, Actions: []PublicHTTPSActionReceipt{}, State: "reviewed", Resources: []PublicHTTPSResource{}, PVCDataRetained: false, Message: "Reviewed only. No infrastructure, DNS record, certificate or publication has been created."}}
	row.Plan = PublicHTTPSPlan{ID: id, ProjectID: p.ID, DeploymentID: input.DeploymentID, Destination: PublicHTTPSDestination{TargetID: selected.ID, TargetFingerprint: selected.Fingerprint, Namespace: selected.Namespace, NamespaceUID: selected.NamespaceUID, ServiceID: service.ID, ServiceName: serviceName, ServiceUID: uid, ServicePort: service.Port}, Hostname: input.Hostname, Path: input.Path, PathType: input.PathType, AzureDNSZoneID: input.AzureDNSZoneID, AzureDNSRecordName: relative, AzureJobID: job.ID, AzureJobPlanDigest: job.Plan.Foundation.PlanDigest, AzureClusterUID: job.Cluster.ResourceUID, TraefikImage: image, Profile: deploymentkube.PublicHTTPSProfile, StorageClass: input.StorageClass, StorageGiB: 1, ACMEEmail: input.ACMEEmail, DNSClientID: input.DNSCredential.ClientID, MonthlyBudgetUSD: input.MonthlyBudgetUSD, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano)}
	if _, e = s.publicHTTPSJob(row); e != nil {
		return PublicHTTPSStatus{}, e
	}
	target, e := s.publicHTTPSTarget(ctx, row)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	actual, e := target.Client.CoreV1().Services(target.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if e != nil || actual == nil || string(actual.UID) != uid || !deploymentkube.PublicHTTPSDestinationMatches(review.Request.Blueprint, actual) {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	class, e := target.Client.StorageV1().StorageClasses().Get(ctx, input.StorageClass, metav1.GetOptions{})
	if e != nil || class == nil || class.UID == "" || class.ResourceVersion == "" || class.DeletionTimestamp != nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPSStorage
	}
	binding := "Immediate"
	if class.VolumeBindingMode != nil {
		binding = string(*class.VolumeBindingMode)
	}
	if binding != "Immediate" && binding != "WaitForFirstConsumer" {
		return PublicHTTPSStatus{}, ErrPublicHTTPSStorage
	}
	row.Plan.StorageClassUID, row.Plan.StorageClassVersion, row.Plan.StorageBindingMode = string(class.UID), class.ResourceVersion, binding
	var dnsZone publicHTTPSZone
	if s.publicHTTPSARMRead(ctx, row, "zone", &dnsZone) != nil || !strings.EqualFold(dnsZone.ID, input.AzureDNSZoneID) || dnsZone.Name != zone[3] || dnsZone.Properties.ZoneType != "Public" || len(dnsZone.Properties.NameServers) < 2 || len(dnsZone.Properties.NameServers) > 8 || !publicHTTPSHeader(dnsZone.ETag) {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	row.Plan.AzureDNSZoneETag = dnsZone.ETag
	if s.publicHTTPSDNSAbsent(ctx, row) != nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	objects, e := deploymentkube.PublicHTTPSObjects(publicHTTPSSpec(row), publicHTTPSCredentialData(row))
	if e != nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	for _, object := range objects {
		if e = s.publicHTTPSRPC(ctx, target, "GET", object.Kind, object.Name, nil, nil); !errors.Is(e, deploymentkube.ErrPreparationAbsent) {
			return PublicHTTPSStatus{}, ErrPublicHTTPS
		}
		row.Resources = append(row.Resources, PublicHTTPSResource{Kind: object.Kind, Name: object.Name, State: "planned"})
	}
	row.Plan.Digest = publicHTTPSPlanDigest(row.Plan)
	if s.publicHTTPSAuthority(ctx, actor, true, input.MonthlyBudgetUSD) != nil || s.authorizeKubernetesReview(ctx, review, false, true) != nil {
		return PublicHTTPSStatus{}, management.ErrExecutionAuthority
	}
	s.publicHTTPSMu.Lock()
	defer s.publicHTTPSMu.Unlock()
	recordPath, e := s.publicHTTPSPath(p, id)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	if _, prior, e := s.readPublicHTTPS(p, id); e == nil {
		if prior.ReviewRequestDigest != requestDigest {
			return PublicHTTPSStatus{}, ErrPublicHTTPS
		}
		if e = s.publicHTTPSRememberLocked(p, id); e != nil {
			return PublicHTTPSStatus{}, e
		}
		return prior.PublicHTTPSStatus, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return PublicHTTPSStatus{}, e
	}
	if !validStoredPublicHTTPS(row, p) || publicHTTPSWrite(recordPath, row, 128<<10) != nil {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	if e = s.publicHTTPSRememberLocked(p, id); e != nil {
		return PublicHTTPSStatus{}, e
	}
	return row.PublicHTTPSStatus, nil
}
func (s *Server) publicHTTPSRPC(ctx context.Context, target deploymentkube.Target, method, kind, name string, input, output any) error {
	if s.publicHTTPSResourceRPC != nil {
		return s.publicHTTPSResourceRPC(ctx, target, method, kind, name, input, output)
	}
	return deploymentkube.PublicHTTPSResourceRPC(ctx, target, method, kind, name, input, output)
}

// Used by final readiness and managed delivery. It compares exact current
// cluster ownership/name/port and never claims app behavior from a Service UID.
func publicHTTPSAppServiceReady(row storedPublicHTTPS, actual *corev1.Service) bool {
	return actual != nil && string(actual.UID) == row.Plan.Destination.ServiceUID && actual.DeletionTimestamp == nil && actual.Spec.Type == corev1.ServiceTypeClusterIP
}

func (s *Server) publicHTTPSDestinationCurrent(ctx context.Context, p project.Project, row storedPublicHTTPS) error {
	target, e := s.publicHTTPSTarget(ctx, row)
	if e != nil {
		return e
	}

	service, e := target.Client.CoreV1().Services(target.Namespace).Get(ctx, row.Plan.Destination.ServiceName, metav1.GetOptions{})
	blueprint, be := s.publicHTTPSCurrentBlueprint(p)
	if e != nil || be != nil || service == nil || string(service.UID) != row.Plan.Destination.ServiceUID || service.DeletionTimestamp != nil || !deploymentkube.PublicHTTPSDestinationMatches(blueprint, service) {
		return ErrPublicHTTPS
	}
	found := false
	for _, v := range blueprint.Services {
		if v.ID == row.Plan.Destination.ServiceID && v.Kind == "app" && v.Port == row.Plan.Destination.ServicePort {
			found = true
		}
	}
	if !found {
		return ErrPublicHTTPS
	}
	if r := blueprint.PublicRoute; r != nil && r.Profile == deploymentplan.ManagedPublicRouteProfile && r.ManagedHTTPSID == row.ID && (r.ServiceID != row.Plan.Destination.ServiceID || r.Hostname != row.Plan.Hostname || r.Path != row.Plan.Path || r.PathType != row.Plan.PathType) {
		return ErrPublicHTTPS
	}
	return s.publicHTTPSAuthority(ctx, row.Actor, false, 0)
}
