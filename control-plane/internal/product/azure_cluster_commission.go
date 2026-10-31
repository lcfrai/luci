package product

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

type azureClusterRoleAssignment struct {
	ID         string `json:"id"`
	Properties struct {
		PrincipalID                        string `json:"principalId"`
		PrincipalType                      string `json:"principalType"`
		RoleDefinitionID                   string `json:"roleDefinitionId"`
		Scope                              string `json:"scope"`
		Description                        string `json:"description"`
		Condition                          string `json:"condition"`
		DelegatedManagedIdentityResourceID string `json:"delegatedManagedIdentityResourceId"`
	} `json:"properties"`
}

func aksRoleID(j CloudJob) string {
	fit := j.Plan.Foundation.Cluster.Fit
	return fit.ClusterResourceID + "/providers/Microsoft.Authorization/roleAssignments/" + deploymentplan.AzureClusterRoleAssignmentName(fit.ClusterResourceID, fit.Principal.ObjectID)
}
func aksRoleMatches(row azureClusterRoleAssignment, j CloudJob) bool {
	fit := j.Plan.Foundation.Cluster.Fit
	p := row.Properties
	return strings.EqualFold(row.ID, aksRoleID(j)) && strings.EqualFold(p.PrincipalID, fit.Principal.ObjectID) && p.PrincipalType == fit.Principal.Kind && strings.EqualFold(p.RoleDefinitionID, fit.RoleDefinitionID) && (p.Scope == "" || strings.EqualFold(p.Scope, fit.ClusterResourceID)) && p.Condition == "" && p.DelegatedManagedIdentityResourceID == ""
}

// A complete successful list proves absence of the EXACT assignment ID; a
// provider error or partial page never proves no assignment. No role is deleted.
func (s *Server) readAKSRole(ctx context.Context, j CloudJob) (*azureClusterRoleAssignment, error) {
	fit := j.Plan.Foundation.Cluster.Fit
	url := "https://management.azure.com" + fit.ClusterResourceID + "/providers/Microsoft.Authorization/roleAssignments?api-version=" + deploymentplan.AzureRoleAssignmentAPIVersion + "&$filter=atScope()"
	raw, err := s.cloudJobCommand(ctx, j, "az", []string{"rest", "--method", "GET", "--url", url, "--output", "json"}, "")
	if err != nil {
		return nil, err
	}
	var page struct {
		Value    []azureClusterRoleAssignment `json:"value"`
		NextLink string                       `json:"nextLink"`
	}
	if decodeBoundedAzureJSON(raw, &page) != nil || page.Value == nil || len(page.Value) > 128 || page.NextLink != "" {
		return nil, errAzureCluster
	}
	var found *azureClusterRoleAssignment
	for _, row := range page.Value {
		if !strings.EqualFold(row.ID, aksRoleID(j)) {
			continue
		}
		if found != nil || !aksRoleMatches(row, j) {
			return nil, errAzureCluster
		}
		copy := row
		found = &copy
	}
	return found, nil
}
func (s *Server) aksCommissionJob(ctx context.Context, actor management.ExecutionActorRef, id string, mutation bool) (CloudJob, error) {
	s.mu.RLock()
	j, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok || j.Status != "resources_ready" || j.DeleteAttempted || j.Kind != "aks" || j.Plan.Foundation == nil || j.Plan.Foundation.Cluster == nil || j.Plan.Foundation.Actor != actor || j.Cluster == nil || j.Cluster.State != "created" || !j.Owned {
		return CloudJob{}, errAzureCluster
	}
	p, err := s.authorizeFoundationPlan(ctx, j.Plan, false, true)
	if err != nil {
		return CloudJob{}, err
	}
	// IAM/namespace commissioning creates no additional public endpoint. The
	// cluster creation itself already has its separate exposure policy gate.
	if mutation {
		if _, err = s.foundationAuthorityProfile(ctx, actor, true, j.Plan.MonthlyBudgetUSD, false, true); err != nil {
			return CloudJob{}, err
		}
		s.mu.RLock()
		err = s.recheckFoundationContextLocked(p, j.Plan)
		s.mu.RUnlock()
		if err != nil {
			return CloudJob{}, err
		}
	}
	current, err := s.observeAzureCluster(ctx, j)
	if err != nil || current.State != "created" || current.ResourceUID != j.Cluster.ResourceUID {
		return CloudJob{}, errAzureCluster
	}
	return j, nil
}
func (s *Server) aksNamespaceClient(ctx context.Context, j CloudJob) (kubernetes.Interface, string, error) {
	b := j.Plan.Foundation
	fit := b.Cluster.Fit
	current, err := s.foundationAuthorityProfile(ctx, b.Actor, false, 0, false, true)
	if err != nil {
		return nil, "", err
	}
	bearer, err := s.azureClusterToken(current, b.Actor, fit.Spec, &fit.Principal)
	if err != nil {
		return nil, "", err
	}
	uri := "https://management.azure.com" + fit.ClusterResourceID + "/listClusterUserCredential?api-version=" + deploymentplan.AzureClusterAPI(fit.Spec) + "&format=exec"
	raw, err := s.cloudJobCommand(ctx, j, "az", []string{"rest", "--method", "POST", "--url", uri, "--output", "json"}, "")
	if err != nil {
		return nil, "", err
	}
	server, ca, err := azureClusterServerCA(raw, j.Cluster.FQDN)
	if err != nil {
		return nil, "", err
	}
	client, err := kubernetesClient(privateKubernetesTarget{KubernetesTargetDescriptor: KubernetesTargetDescriptor{Server: server}, Credentials: KubernetesTargetCredentials{CA: ca, Token: bearer.token}})
	if err != nil || !time.Now().Before(bearer.expires.Add(-time.Minute)) {
		return nil, "", errAzureCluster
	}
	return client, ca, nil
}

// An unresolved original Create is never repeated. A new explicit import may
// proceed only after access is positively observed; it retains the old attempt.
func aksCommissionReviewBlocked(old AzureClusterCommission, mode string, accessObserved bool) bool {
	if old.NamespaceAttempted && old.NamespaceUID == "" {
		return mode != "existing" || !accessObserved
	}
	return old.RoleAttempted && !old.RoleObserved && (mode != "existing" || !accessObserved)
}
func namespaceForCommission(object *corev1.Namespace, row AzureClusterCommission) bool {
	if object == nil || row.NamespaceUID != "" && string(object.UID) != row.NamespaceUID || object.Name != row.Namespace || object.UID == "" || len(object.UID) > 128 || object.ResourceVersion == "" || object.DeletionTimestamp != nil {
		return false
	}
	if row.Mode == "existing" {
		return string(object.UID) == row.ExpectedNamespaceUID && (row.NamespaceUID != "" && string(object.UID) == row.NamespaceUID || row.NamespaceUID == "" && object.ResourceVersion == row.ExpectedNamespaceVersion)
	}
	return object.Labels["app.kubernetes.io/managed-by"] == "luci-cluster-commission" && object.Labels["luci.ai/commission"] == row.ID && object.Labels["luci.ai/project"] == reviewDigest(row.ProjectID)[:12] && object.Annotations["luci.ai/cluster-resource-uid"] == row.Cluster.ResourceUID
}
func (s *Server) ReviewAzureClusterCommission(ctx context.Context, actor management.ExecutionActorRef, req AzureClusterCommissionRequest) (AzureClusterCommission, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if !prototypeReleaseID.MatchString(req.JobID) || len(kubevalidation.IsDNS1123Label(req.Namespace)) != 0 || req.Mode != "create" && req.Mode != "existing" || req.Mode == "create" && (req.Namespace == "default" || strings.HasPrefix(req.Namespace, "kube-")) {
		return AzureClusterCommission{}, errAzureCluster
	}
	j, err := s.aksCommissionJob(ctx, actor, req.JobID, false)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	// Reviewing a new bootstrap effect also requires current source/saved intent,
	// but does not allocate or pretend to verify a price.
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	s.mu.RLock()
	err = s.recheckFoundationContextLocked(p, j.Plan)
	s.mu.RUnlock()
	if err != nil {
		return AzureClusterCommission{}, err
	}
	role, err := s.readAKSRole(ctx, j)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	id, err := kubeOpaqueID("aks-")
	if err != nil {
		return AzureClusterCommission{}, err
	}
	targetID, err := kubeOpaqueID("cluster-")
	if err != nil {
		return AzureClusterCommission{}, err
	}
	now := time.Now().UTC()
	fit := j.Plan.Foundation.Cluster.Fit
	row := AzureClusterCommission{ID: id, ProjectID: actor.ProjectID, Revision: 1, Actor: actor, JobID: j.ID, PlanDigest: j.Plan.Foundation.PlanDigest, Cluster: *j.Cluster, Namespace: req.Namespace, Mode: req.Mode, Principal: fit.Principal, RoleDefinitionID: fit.RoleDefinitionID, RoleAssignmentID: aksRoleID(j), RoleExistingAtReview: role != nil, RoleObserved: role != nil, TargetID: targetID, MonthlyBudgetUSD: j.Plan.MonthlyBudgetUSD, State: "reviewed", CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano), ObservedAt: now.Format(time.RFC3339Nano)}
	if req.Mode == "existing" {
		// Existing namespace import is never inferred from an IAM row alone. Real
		// current credentials must read its exact UID/version; no mutation occurs.
		client, _, err := s.aksNamespaceClient(ctx, j)
		if err != nil {
			return AzureClusterCommission{}, err
		}
		object, err := client.CoreV1().Namespaces().Get(ctx, req.Namespace, metav1.GetOptions{})
		if err != nil || object == nil || object.UID == "" || object.DeletionTimestamp != nil {
			return AzureClusterCommission{}, errAzureCluster
		}
		row.ExpectedNamespaceUID, row.ExpectedNamespaceVersion = string(object.UID), object.ResourceVersion
	} else if role != nil {
		client, _, err := s.aksNamespaceClient(ctx, j)
		if err != nil {
			return AzureClusterCommission{}, err
		}
		object, readErr := client.CoreV1().Namespaces().Get(ctx, req.Namespace, metav1.GetOptions{})
		if readErr == nil || object != nil || !apierrors.IsNotFound(readErr) {
			return AzureClusterCommission{}, errAzureCluster
		}
	}
	if _, err = s.aksCommissionJob(ctx, actor, j.ID, false); err != nil {
		return AzureClusterCommission{}, err
	}
	saved, saveErr := func() (AzureClusterCommission, error) {
		s.kubernetesReviewMu.Lock()
		defer s.kubernetesReviewMu.Unlock()
		path, ledger, err := s.readAKSCommissions(actor.ProjectID)
		if err != nil {
			return AzureClusterCommission{}, err
		}
		for _, old := range ledger.Rows {
			if old.Actor != actor || old.JobID != j.ID || old.Namespace != row.Namespace || old.Cluster.ResourceUID != row.Cluster.ResourceUID {
				continue
			}
			expiry, _ := time.Parse(time.RFC3339Nano, old.ExpiresAt)
			if aksCommissionReviewBlocked(old, req.Mode, role != nil) {
				return AzureClusterCommission{}, errAzureCluster
			}
			if old.Mode == row.Mode && old.PlanDigest == row.PlanDigest && old.RoleExistingAtReview == row.RoleExistingAtReview && old.ExpectedNamespaceUID == row.ExpectedNamespaceUID && old.ExpectedNamespaceVersion == row.ExpectedNamespaceVersion && !old.RoleAttempted && !old.NamespaceAttempted && now.Before(expiry) {
				return old, nil
			}
		}
		if len(ledger.Rows) >= azureClusterCommissionLimit {
			return AzureClusterCommission{}, errAzureCluster
		}
		ledger.Rows = append(ledger.Rows, row)
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return AzureClusterCommission{}, management.ErrExecutionAuthority
		}
		if err = s.saveAKSCommission(path, ledger); err != nil {
			return AzureClusterCommission{}, err
		}
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return AzureClusterCommission{}, management.ErrExecutionAuthority
		}
		return row, nil
	}()
	if saveErr != nil {
		return AzureClusterCommission{}, saveErr
	}
	// The canonical late-grant read reacquires the mutex only AFTER reservation
	// and persistence released it, for both a new row and an exact duplicate.
	return s.aksCommission(ctx, actor, saved.ID)
}

// This last local fence follows provider/journal I/O. No source/permission claim
// is retained merely because an earlier review succeeded.
func (s *Server) authorizeAKSCommissionWrite(ctx context.Context, row AzureClusterCommission, j CloudJob) error {
	if row.Actor != j.Plan.Foundation.Actor || row.PlanDigest != j.Plan.Foundation.PlanDigest || row.Cluster.ResourceUID != j.Cluster.ResourceUID {
		return errAzureCluster
	}
	expiry, err := time.Parse(time.RFC3339Nano, row.ExpiresAt)
	if err != nil || !time.Now().Before(expiry) {
		return errAzureCluster
	}
	if _, err = s.authorizeFoundationPlan(ctx, j.Plan, false, true); err != nil {
		return err
	}
	if _, err = s.foundationAuthorityProfile(ctx, row.Actor, true, row.MonthlyBudgetUSD, false, true); err != nil {
		return err
	}
	p, err := s.resolveProject(row.ProjectID)
	if err != nil {
		return err
	}
	s.mu.RLock()
	err = s.recheckFoundationContextLocked(p, j.Plan)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return s.kubernetesAuthority(ctx, row.Actor, true, true, row.MonthlyBudgetUSD)
}

// action is one of two distinct reviewed effects. Existing/unknown attempts
// ONLY Observe; neither automatic retries nor a replacement operation is minted.
func (s *Server) ApplyAzureClusterCommission(ctx context.Context, actor management.ExecutionActorRef, id, action string, costAck, accessAck bool) (AzureClusterCommission, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	row, err := s.aksCommission(ctx, actor, id)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	if action != "access" && action != "namespace" || !costAck {
		return AzureClusterCommission{}, errAzureCluster
	}
	if action == "access" && (row.RoleAttempted || row.RoleExistingAtReview) || action == "namespace" && row.NamespaceAttempted {
		return s.ObserveAzureClusterCommission(ctx, actor, id)
	}
	j, err := s.aksCommissionJob(ctx, actor, row.JobID, true)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	if err = s.authorizeAKSCommissionWrite(ctx, row, j); err != nil {
		return AzureClusterCommission{}, err
	}
	if action == "access" {
		if !accessAck {
			return AzureClusterCommission{}, errAzureCluster
		}
		existing, err := s.readAKSRole(ctx, j)
		if err != nil || existing != nil {
			return AzureClusterCommission{}, errAzureCluster
		}
		row.RoleAttempted = true
		row.State = "access-unknown"
		before := row.Revision
		if err = s.updateAKSCommission(ctx, actor, row, before); err != nil {
			return AzureClusterCommission{}, err
		}
		row.Revision++
		j, err = s.aksCommissionJob(ctx, actor, row.JobID, true)
		if err != nil {
			return AzureClusterCommission{}, err
		}
		if err = s.authorizeAKSCommissionWrite(ctx, row, j); err != nil {
			return AzureClusterCommission{}, err
		}
		body, _ := json.Marshal(map[string]any{"properties": map[string]string{"principalId": row.Principal.ObjectID, "principalType": row.Principal.Kind, "roleDefinitionId": row.RoleDefinitionID, "description": "Luci reviewed bootstrap " + row.ID}})
		uri := "https://management.azure.com" + row.RoleAssignmentID + "?api-version=" + deploymentplan.AzureRoleAssignmentAPIVersion
		// Exact scope, role and principal only. No fallback to a parent/subscription
		// scope, other principal, admin credentials or implicit role assignment.
		if _, err = s.cloudJobCommandBefore(ctx, j, "az", []string{"rest", "--method", "PUT", "--url", uri, "--body", string(body), "--output", "json"}, "", func(current context.Context) error {
			latest, err := s.aksCommissionJob(current, actor, row.JobID, true)
			if err != nil {
				return err
			}
			return s.authorizeAKSCommissionWrite(current, row, latest)
		}); err != nil {
			return AzureClusterCommission{}, errAzureCluster
		}
		return s.ObserveAzureClusterCommission(ctx, actor, id)
	}
	role, err := s.readAKSRole(ctx, j)
	if err != nil || role == nil {
		return AzureClusterCommission{}, errAzureCluster
	}
	if !row.RoleExistingAtReview && role.Properties.Description != "Luci reviewed bootstrap "+row.ID {
		return AzureClusterCommission{}, errAzureCluster
	}
	client, _, err := s.aksNamespaceClient(ctx, j)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	current, readErr := client.CoreV1().Namespaces().Get(ctx, row.Namespace, metav1.GetOptions{})
	if row.Mode == "existing" {
		if readErr != nil || !namespaceForCommission(current, row) {
			return AzureClusterCommission{}, errAzureCluster
		}
	} else if readErr == nil || current != nil || !apierrors.IsNotFound(readErr) {
		return AzureClusterCommission{}, errAzureCluster
	}
	row.NamespaceAttempted = true
	row.State = "namespace-unknown"
	before := row.Revision
	if err = s.updateAKSCommission(ctx, actor, row, before); err != nil {
		return AzureClusterCommission{}, err
	}
	row.Revision++
	j, err = s.aksCommissionJob(ctx, actor, row.JobID, true)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	client, _, err = s.aksNamespaceClient(ctx, j)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	j, err = s.aksCommissionJob(ctx, actor, row.JobID, true)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	if err = s.authorizeAKSCommissionWrite(ctx, row, j); err != nil {
		return AzureClusterCommission{}, err
	}
	if row.Mode == "create" {
		object := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: row.Namespace, Labels: map[string]string{"app.kubernetes.io/managed-by": "luci-cluster-commission", "luci.ai/commission": row.ID, "luci.ai/project": reviewDigest(row.ProjectID)[:12]}, Annotations: map[string]string{"luci.ai/cluster-resource-uid": row.Cluster.ResourceUID}}}
		// Namespace Create is name-conditional, not a transaction with ARM cluster
		// ResourceUID. Fresh before/after observations detect replacement; unknown
		// identities are withheld and never silently adopted or replayed.
		created, err := client.CoreV1().Namespaces().Create(ctx, object, metav1.CreateOptions{})
		if err != nil || !namespaceForCommission(created, row) {
			return AzureClusterCommission{}, errAzureCluster
		}
		// A positively returned UID is the original identity, not disposable
		// response data. Retain it before any later observation can see a replacement.
		row.NamespaceUID = string(created.UID)
		before := row.Revision
		if err = s.updateAKSCommission(ctx, actor, row, before); err != nil {
			return AzureClusterCommission{}, err
		}
		row.Revision++
	}
	return s.ObserveAzureClusterCommission(ctx, actor, id)
}

func (s *Server) ObserveAzureClusterCommission(ctx context.Context, actor management.ExecutionActorRef, id string) (AzureClusterCommission, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	row, err := s.aksCommission(ctx, actor, id)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	j, err := s.aksCommissionJob(ctx, actor, row.JobID, false)
	if err != nil || j.Cluster.ResourceUID != row.Cluster.ResourceUID {
		return AzureClusterCommission{}, errAzureCluster
	}
	role, err := s.readAKSRole(ctx, j)
	if err != nil {
		return AzureClusterCommission{}, err
	}
	row.RoleObserved = role != nil && (row.RoleExistingAtReview || role.Properties.Description == "Luci reviewed bootstrap "+row.ID)
	if !row.RoleObserved {
		if row.RoleAttempted {
			row.State = "access-unknown"
		} else {
			row.State = "reviewed"
		}
	} else if !row.NamespaceAttempted {
		row.State = "access-observed"
	} else if row.Mode == "create" && row.NamespaceUID == "" {
		// A lost Create response has no retained original UID. Matching labels alone
		// cannot promote it to owned/connected. A separate explicit existing-mode
		// review may import a freshly read UID without changing/replaying this attempt.
		row.State = "namespace-unknown"
	} else {
		client, ca, err := s.aksNamespaceClient(ctx, j)
		if err != nil {
			return AzureClusterCommission{}, err
		}
		object, err := client.CoreV1().Namespaces().Get(ctx, row.Namespace, metav1.GetOptions{})
		if err != nil || !namespaceForCommission(object, row) || row.NamespaceUID != "" && row.NamespaceUID != string(object.UID) {
			return AzureClusterCommission{}, errAzureCluster
		}
		if _, err = s.aksCommissionJob(ctx, actor, row.JobID, false); err != nil {
			return AzureClusterCommission{}, err
		}
		row.NamespaceUID = string(object.UID)
		if err = s.sealAzureClusterTarget(ctx, row, j, ca); err != nil {
			return AzureClusterCommission{}, err
		}
		object, err = client.CoreV1().Namespaces().Get(ctx, row.Namespace, metav1.GetOptions{})
		if err != nil || !namespaceForCommission(object, row) || string(object.UID) != row.NamespaceUID {
			return AzureClusterCommission{}, errAzureCluster
		}
		row.State = "connected"
	}
	if _, err = s.aksCommissionJob(ctx, actor, row.JobID, false); err != nil {
		return AzureClusterCommission{}, err
	}
	before := row.Revision
	if err = s.updateAKSCommission(ctx, actor, row, before); err != nil {
		return AzureClusterCommission{}, err
	}
	row.Revision++
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return AzureClusterCommission{}, management.ErrExecutionAuthority
	}
	return s.aksCommission(ctx, actor, id)
}
