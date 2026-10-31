package product

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

const azureClusterCommissionLimit = 64
const azureClusterCommissionBytes = 4 << 20

type AzureClusterCommissionRequest struct {
	JobID     string `json:"jobId"`
	Namespace string `json:"namespace"`
	Mode      string `json:"mode"`
}
type AzureClusterCommission struct {
	ID                       string                               `json:"id"`
	ProjectID                string                               `json:"projectId"`
	Revision                 int                                  `json:"revision"`
	Actor                    management.ExecutionActorRef         `json:"actor"`
	JobID                    string                               `json:"jobId"`
	PlanDigest               string                               `json:"planDigest"`
	Cluster                  AzureClusterObservation              `json:"cluster"`
	Namespace                string                               `json:"namespace"`
	Mode                     string                               `json:"mode"`
	ExpectedNamespaceUID     string                               `json:"expectedNamespaceUid"`
	ExpectedNamespaceVersion string                               `json:"expectedNamespaceVersion"`
	Principal                deploymentplan.AzureClusterPrincipal `json:"principal"`
	RoleDefinitionID         string                               `json:"roleDefinitionId"`
	RoleAssignmentID         string                               `json:"roleAssignmentId"`
	RoleAttempted            bool                                 `json:"roleAttempted"`
	RoleObserved             bool                                 `json:"roleObserved"`
	RoleExistingAtReview     bool                                 `json:"roleExistingAtReview"`
	NamespaceAttempted       bool                                 `json:"namespaceAttempted"`
	NamespaceUID             string                               `json:"namespaceUid"`
	TargetID                 string                               `json:"targetId"`
	MonthlyBudgetUSD         float64                              `json:"monthlyBudgetUsd"`
	State                    string                               `json:"state"`
	CreatedAt                string                               `json:"createdAt"`
	ExpiresAt                string                               `json:"expiresAt"`
	ObservedAt               string                               `json:"observedAt"`
}
type azureClusterCommissionLedger struct {
	Version      int                      `json:"version"`
	ProjectID    string                   `json:"projectId"`
	AccountScope string                   `json:"accountScope"`
	ProjectScope string                   `json:"projectScope"`
	Rows         []AzureClusterCommission `json:"rows"`
}

func validAKSCommission(row AzureClusterCommission, projectID string) bool {
	if !validAKSCommissionID(row.ID) || row.ProjectID != projectID || row.Actor.Local || row.Actor.UserID == "" || row.Actor.ProjectID != projectID || row.Revision < 1 || row.Revision > 1_000_000_000 || !prototypeReleaseID.MatchString(row.JobID) || !hexDigest(row.PlanDigest) || !deploymentplan.ValidateAzureClusterPrincipal(row.Principal) || row.RoleDefinitionID != "/subscriptions/"+row.Principal.SubscriptionID+"/providers/Microsoft.Authorization/roleDefinitions/"+deploymentplan.AzureAKSClusterAdminRole || row.RoleAssignmentID != row.Cluster.ResourceID+"/providers/Microsoft.Authorization/roleAssignments/"+deploymentplan.AzureClusterRoleAssignmentName(row.Cluster.ResourceID, row.Principal.ObjectID) || row.Cluster.ResourceUID == "" || len(row.Cluster.ResourceUID) > 128 || row.Cluster.TargetReady || row.Cluster.CapacityVerified || row.Cluster.State != "created" || len(kubevalidation.IsDNS1123Label(row.Namespace)) != 0 || strings.ContainsAny(row.Namespace, "/\\. \r\n\t") || !validKubeTargetID(row.TargetID) || math.IsNaN(row.MonthlyBudgetUSD) || math.IsInf(row.MonthlyBudgetUSD, 0) || row.MonthlyBudgetUSD <= 0 || row.MonthlyBudgetUSD > 100000 {
		return false
	}
	if row.Mode != "create" && row.Mode != "existing" || row.Mode == "create" && (row.ExpectedNamespaceUID != "" || row.ExpectedNamespaceVersion != "") || row.Mode == "existing" && (row.ExpectedNamespaceUID == "" || row.ExpectedNamespaceVersion == "") || len(row.ExpectedNamespaceUID) > 128 || len(row.ExpectedNamespaceVersion) > 128 || len(row.NamespaceUID) > 128 {
		return false
	}
	created, e1 := time.Parse(time.RFC3339Nano, row.CreatedAt)
	expiry, e2 := time.Parse(time.RFC3339Nano, row.ExpiresAt)
	_, e3 := time.Parse(time.RFC3339Nano, row.ObservedAt)
	if e1 != nil || e2 != nil || e3 != nil || !expiry.Equal(created.Add(10*time.Minute)) {
		return false
	}
	switch row.State {
	case "reviewed", "access-unknown", "access-observed", "namespace-unknown", "connected", "blocked":
	default:
		return false
	}
	return row.State != "connected" || row.NamespaceUID != ""
}
func validAKSCommissionID(value string) bool {
	return len(value) == 36 && strings.HasPrefix(value, "aks-") && hexDigest(value[4:]+strings.Repeat("0", 32))
}
func validKubeTargetID(value string) bool {
	return len(value) == 40 && strings.HasPrefix(value, "cluster-") && hexDigest(value[8:]+strings.Repeat("0", 32))
}
func (s *Server) readAKSCommissions(projectID string) (string, azureClusterCommissionLedger, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", azureClusterCommissionLedger{}, err
	}
	s.mu.RLock()
	scope, err := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return "", azureClusterCommissionLedger{}, err
	}
	empty := azureClusterCommissionLedger{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Rows: []AzureClusterCommission{}}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "azure-cluster-commissions-"+projectID)
	if err != nil {
		return "", empty, err
	}
	var stored azureClusterCommissionLedger
	err = externalReadJSON(path, azureClusterCommissionBytes, &stored)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || stored.Version != 1 || stored.ProjectID != p.ID || stored.AccountScope != empty.AccountScope || stored.ProjectScope != empty.ProjectScope || stored.Rows == nil || len(stored.Rows) > azureClusterCommissionLimit {
		return "", empty, errAzureCluster
	}
	seen := map[string]bool{}
	for _, row := range stored.Rows {
		if !validAKSCommission(row, p.ID) || seen[row.ID] {
			return "", empty, errAzureCluster
		}
		seen[row.ID] = true
	}
	return path, stored, nil
}

// Caller holds the existing separate review mutex; no provider I/O while held.
func (s *Server) saveAKSCommission(path string, ledger azureClusterCommissionLedger) error {
	if ledger.Version != 1 || ledger.Rows == nil || len(ledger.Rows) > azureClusterCommissionLimit {
		return errAzureCluster
	}
	for _, row := range ledger.Rows {
		if !validAKSCommission(row, ledger.ProjectID) {
			return errAzureCluster
		}
	}
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(raw)+1 > azureClusterCommissionBytes {
		return errAzureCluster
	}
	return durableRunJSON(path, ledger)
}
func (s *Server) aksCommission(ctx context.Context, actor management.ExecutionActorRef, id string) (AzureClusterCommission, error) {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return AzureClusterCommission{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readAKSCommissions(actor.ProjectID)
	s.kubernetesReviewMu.Unlock()
	if err != nil {
		return AzureClusterCommission{}, err
	}
	for _, row := range ledger.Rows {
		if row.ID == id && row.Actor == actor {
			if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
				return AzureClusterCommission{}, management.ErrExecutionAuthority
			}
			return row, nil
		}
	}
	return AzureClusterCommission{}, errAzureCluster
}
func aksCommissionIntentDigest(row AzureClusterCommission) string {
	row.Revision = 0
	row.RoleAttempted = false
	row.RoleObserved = false
	row.NamespaceAttempted = false
	row.NamespaceUID = ""
	row.State = ""
	row.ObservedAt = ""
	return reviewDigest(row)
}
func (s *Server) updateAKSCommission(ctx context.Context, actor management.ExecutionActorRef, next AzureClusterCommission, expected int) error {
	if next.Actor != actor || next.ProjectID != actor.ProjectID || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	defer s.kubernetesReviewMu.Unlock()
	path, ledger, err := s.readAKSCommissions(actor.ProjectID)
	if err != nil {
		return err
	}
	for i, row := range ledger.Rows {
		if row.ID != next.ID {
			continue
		}
		if row.Actor != actor || row.Revision != expected || aksCommissionIntentDigest(row) != aksCommissionIntentDigest(next) || row.PlanDigest != next.PlanDigest || row.JobID != next.JobID || row.Cluster.ResourceUID != next.Cluster.ResourceUID || row.Namespace != next.Namespace || row.Mode != next.Mode || row.TargetID != next.TargetID || row.Principal != next.Principal || row.RoleAttempted && !next.RoleAttempted || row.NamespaceAttempted && !next.NamespaceAttempted || row.NamespaceUID != "" && row.NamespaceUID != next.NamespaceUID {
			return errAzureCluster
		}
		next.Revision = expected + 1
		next.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
		ledger.Rows[i] = next
		if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
			return management.ErrExecutionAuthority
		}
		return s.saveAKSCommission(path, ledger)
	}
	return errAzureCluster
}
func (s *Server) ListAzureClusterCommissions(ctx context.Context, actor management.ExecutionActorRef) ([]AzureClusterCommission, error) {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readAKSCommissions(actor.ProjectID)
	s.kubernetesReviewMu.Unlock()
	if err != nil {
		return nil, err
	}
	rows := []AzureClusterCommission{}
	for _, row := range ledger.Rows {
		if row.Actor == actor {
			rows = append(rows, row)
		}
	}
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	return rows, nil
}
