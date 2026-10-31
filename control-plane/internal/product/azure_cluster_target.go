package product

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// A private source descriptor, never a bearer or kubeconfig. Team use still
// requires the CURRENT caller's Secrets+Deploy grant and the exact recorded
// Azure account/principal. The resource UID prevents ARM-name reuse adoption.
type AzureClusterCredentialSource struct {
	JobID      string                               `json:"jobId"`
	PlanDigest string                               `json:"planDigest"`
	Cluster    AzureClusterObservation              `json:"cluster"`
	Principal  deploymentplan.AzureClusterPrincipal `json:"principal"`
}

func validateKubernetesTargetCredentials(target privateKubernetesTarget) error {
	if target.AzureCluster == nil {
		if target.CredentialKind != "" {
			return ErrKubernetesReview
		}
		_, err := kubernetesRESTConfig(target.Server, target.Credentials)
		return err
	}
	source := target.AzureCluster
	if target.CredentialKind != "azure-aks-entra" || !prototypeReleaseID.MatchString(source.JobID) || !hexDigest(source.PlanDigest) || !deploymentplan.ValidateAzureClusterPrincipal(source.Principal) || source.Cluster.State != "created" || source.Cluster.ResourceUID == "" || len(source.Cluster.ResourceUID) > 128 || source.Cluster.TargetReady || source.Cluster.CapacityVerified || target.Credentials.Token != "" || target.Credentials.Certificate != "" || target.Credentials.Key != "" {
		return ErrKubernetesReview
	}
	_, err := kubernetesRESTConfig(target.Server, KubernetesTargetCredentials{CA: target.Credentials.CA, Token: "validation-only-not-an-access-token"})
	return err
}

type kubernetesResolutionKey struct{}
type azureClusterResolved struct {
	client    kubernetes.Interface
	principal deploymentplan.AzureClusterPrincipal
	expires   time.Time
	created   time.Time
}
type kubernetesResolutionCache struct {
	mu   sync.Mutex
	rows map[string]azureClusterResolved
}

// WithKubernetesTargetResolution permits transient credential reuse within ONE
// API operation. It is neither a Server cache nor a retained login. Every use
// still repeats current project access, account and immutable cluster checks.
func WithKubernetesTargetResolution(ctx context.Context) context.Context {
	if _, ok := ctx.Value(kubernetesResolutionKey{}).(*kubernetesResolutionCache); ok {
		return ctx
	}
	return context.WithValue(ctx, kubernetesResolutionKey{}, &kubernetesResolutionCache{rows: map[string]azureClusterResolved{}})
}
func (s *Server) confirmAzureClusterAccount(ctx context.Context, actor management.ExecutionActorRef, expected deploymentplan.AzureClusterPrincipal) error {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return management.ErrExecutionAuthority
	}
	raw, err := s.runner(ctx, "az", []string{"account", "show", "--subscription", expected.SubscriptionID, "--output", "json"}, "")
	if err != nil || len(raw) > 64<<10 || ctx.Err() != nil {
		return ErrKubernetesReview
	}
	var account azureAccountMetadata
	if json.Unmarshal([]byte(raw), &account) != nil {
		return ErrKubernetesReview
	}
	current, err := azureAccountPrincipal(account)
	expected.ObjectID = ""
	if err != nil || current != expected || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return management.ErrExecutionAuthority
	}
	return nil
}
func (s *Server) resolvedKubernetesClient(ctx context.Context, identity deploymentkube.Identity, target privateKubernetesTarget) (kubernetes.Interface, error) {
	if target.AzureCluster == nil {
		return kubernetesClient(target)
	}
	cache, _ := ctx.Value(kubernetesResolutionKey{}).(*kubernetesResolutionCache)
	if cache != nil {
		cache.mu.Lock()
		defer cache.mu.Unlock()
	}
	actor, err := management.CaptureDeploymentActor(ctx, identity.ProjectID)
	if err != nil || actor.UserID != identity.ActorID || actor.TeamID != identity.TeamID || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil || validateKubernetesTargetCredentials(target) != nil {
		if cache != nil {
			cache.rows = map[string]azureClusterResolved{}
		}
		return nil, management.ErrExecutionAuthority
	}
	source := target.AzureCluster
	s.mu.RLock()
	j, ok := s.jobs[source.JobID]
	s.mu.RUnlock()
	if !ok || j.Status != "resources_ready" || j.DeleteAttempted || !j.Owned || j.Kind != "aks" || j.Plan.Foundation == nil || j.Plan.Foundation.Cluster == nil || j.Plan.Foundation.Actor.ProjectID != actor.ProjectID || j.Plan.Foundation.PlanDigest != source.PlanDigest || j.Plan.Foundation.Cluster.Fit.Principal != source.Principal || j.Cluster == nil || j.Cluster.ResourceUID != source.Cluster.ResourceUID || !validFoundationPlan(j.Plan) {
		return nil, ErrKubernetesReview
	}
	fit := j.Plan.Foundation.Cluster.Fit
	key := reviewDigest(struct {
		Actor       management.ExecutionActorRef
		Fingerprint string
	}{actor, target.Fingerprint})
	var cached azureClusterResolved
	if cache != nil {
		cached = cache.rows[key]
	}
	usable := cached.client != nil && cached.principal == source.Principal && time.Since(cached.created) >= 0 && time.Since(cached.created) < 30*time.Second && time.Now().Before(cached.expires.Add(-2*time.Minute))
	if !usable && cache != nil {
		delete(cache.rows, key)
	}
	// This explicit project credential source is usable only by a fresh Secrets+
	// Deploy caller. The original creator's identity is not impersonated. A cached
	// bearer keeps its original OID; changing the active account invalidates it.
	if err = s.confirmAzureClusterAccount(ctx, actor, source.Principal); err != nil {
		if cache != nil {
			delete(cache.rows, key)
		}
		return nil, err
	}
	read := func(args ...string) (string, error) {
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return "", management.ErrExecutionAuthority
		}
		raw, err := s.runner(ctx, "az", append(args, "--subscription", fit.Spec.SubscriptionID), "")
		if err != nil || len(raw) > 256<<10 || ctx.Err() != nil {
			return "", ErrKubernetesReview
		}
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return "", management.ErrExecutionAuthority
		}
		return raw, nil
	}
	confirm := func() error {
		raw, err := read("resource", "show", "--ids", fit.ClusterResourceID, "--api-version", deploymentplan.AzureClusterAPI(fit.Spec), "--output", "json")
		if err != nil {
			return err
		}
		observed, err := azureClusterObservation(raw, j)
		if err != nil || observed.State != "created" || observed.ResourceUID != source.Cluster.ResourceUID || observed.PrincipalID != source.Cluster.PrincipalID || observed.FQDN != source.Cluster.FQDN {
			return ErrKubernetesReview
		}
		return nil
	}
	if err := confirm(); err != nil {
		if cache != nil {
			delete(cache.rows, key)
		}
		return nil, err
	}
	if usable {
		if _, err = s.azureClusterToken(ctx, actor, fit.Spec, &source.Principal); err != nil {
			delete(cache.rows, key)
			return nil, err
		}
		return cached.client, nil
	}
	bearer, err := s.azureClusterToken(ctx, actor, fit.Spec, &source.Principal)
	if err != nil {
		return nil, err
	}
	uri := "https://management.azure.com" + fit.ClusterResourceID + "/listClusterUserCredential?api-version=" + deploymentplan.AzureClusterAPI(fit.Spec) + "&format=exec"
	raw, err := read("rest", "--method", "POST", "--url", uri, "--output", "json")
	if err != nil {
		return nil, err
	}
	server, ca, err := azureClusterServerCA(raw, source.Cluster.FQDN)
	if err != nil || server != target.Server || ca != target.Credentials.CA {
		return nil, ErrKubernetesReview
	}
	if err := confirm(); err != nil {
		return nil, err
	}
	if err = s.confirmAzureClusterAccount(ctx, actor, source.Principal); err != nil {
		return nil, err
	}
	if !time.Now().Before(bearer.expires.Add(-2 * time.Minute)) {
		return nil, ErrKubernetesReview
	}
	ephemeral := target
	ephemeral.Credentials.Token = bearer.token
	client, err := kubernetesClient(ephemeral)
	if err != nil {
		return nil, err
	}
	if cache != nil && len(cache.rows) < 8 {
		cache.rows[key] = azureClusterResolved{client: client, principal: source.Principal, expires: bearer.expires, created: time.Now()}
	}
	return client, nil
}

func (s *Server) sealAzureClusterTarget(ctx context.Context, row AzureClusterCommission, j CloudJob, ca string) error {
	if !row.RoleObserved || !row.NamespaceAttempted || row.NamespaceUID == "" || row.Actor != j.Plan.Foundation.Actor || row.Cluster.ResourceUID != j.Cluster.ResourceUID {
		return ErrKubernetesReview
	}
	target := privateKubernetesTarget{KubernetesTargetDescriptor: KubernetesTargetDescriptor{ID: row.TargetID, ProjectID: row.ProjectID, Name: "AKS " + j.Name + " / " + row.Namespace, Server: "https://" + row.Cluster.FQDN, Namespace: row.Namespace, NamespaceUID: row.NamespaceUID, Revision: 1, CredentialKind: "azure-aks-entra"}, Credentials: KubernetesTargetCredentials{CA: ca}, AzureCluster: &AzureClusterCredentialSource{JobID: j.ID, PlanDigest: row.PlanDigest, Cluster: row.Cluster, Principal: row.Principal}}
	if !validKubeTarget(target) || validateKubernetesTargetCredentials(target) != nil {
		return ErrKubernetesReview
	}
	target.Fingerprint = reviewDigest(target)
	// No token is written. Fingerprint includes exact original cluster/account,
	// CA, namespace UID and credential-source revision.
	s.kubernetesTargetMu.Lock()
	defer s.kubernetesTargetMu.Unlock()
	path, ledger, err := s.readKubernetesTargets(row.ProjectID)
	if err != nil {
		return err
	}
	for _, old := range ledger.Targets {
		if old.ID == row.TargetID {
			if old.Fingerprint != target.Fingerprint {
				return ErrKubernetesReview
			}
			return s.kubernetesAuthority(ctx, row.Actor, false, true, 0)
		}
	}
	if len(ledger.Targets) >= 32 {
		return ErrKubernetesReview
	}
	ledger.Targets = append(ledger.Targets, target)
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(raw)+1 > 4<<20 {
		return ErrKubernetesReview
	}
	if s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		return management.ErrExecutionAuthority
	}
	if err = durableRunJSON(path, ledger); err != nil {
		return err
	}
	// The saved target is retained if this late read refuses; not rolled back or
	// certified usable merely because publication succeeded.
	identity := deploymentkube.Identity{ProjectID: row.ProjectID, ActorID: row.Actor.UserID, TeamID: row.Actor.TeamID, TargetID: row.TargetID, TargetFingerprint: target.Fingerprint}
	client, err := s.resolvedKubernetesClient(ctx, identity, target)
	if err != nil {
		return err
	}
	ns, err := client.CoreV1().Namespaces().Get(ctx, row.Namespace, metav1.GetOptions{})
	if err != nil || !namespaceForCommission(ns, row) || string(ns.UID) != row.NamespaceUID {
		return ErrKubernetesReview
	}
	return s.kubernetesAuthority(ctx, row.Actor, false, true, 0)
}

// Source-created targets cannot be silently edited into a static credential
// source by a client. A distinct explicitly configured target may still be saved.
func isAzureClusterTarget(target privateKubernetesTarget) bool {
	return target.AzureCluster != nil || strings.HasPrefix(target.CredentialKind, "azure-")
}
