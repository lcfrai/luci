package product

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Metadata-only grant/descriptor check for long streams. It does not renew a
// login/token, scan source, contact a provider or change an original deadline.
func (s *Server) runtimeResourceMetadataAuthority(ctx context.Context, row storedRuntimePreparation, mutation bool) error {
	if !validRuntimeBlobBinding(row) {
		return ErrRuntimePreparation
	}
	if mutation {
		if err := s.runtimeBlobCurrent(ctx, row, false); err != nil {
			return err
		}
	}
	if row.Plan.Resources == nil {
		return ErrRuntimePreparation
	}
	if err := s.kubernetesAuthority(ctx, row.Plan.Actor, mutation, true, row.Plan.MonthlyBudgetUSD); err != nil {
		return err
	}
	if mutation {
		expiry, err := time.Parse(time.RFC3339Nano, row.Plan.ExpiresAt)
		if err != nil || !time.Now().Before(expiry) {
			return ErrRuntimePreparation
		}
	}
	_, targets, err := s.readKubernetesTargets(row.Plan.ProjectID)
	if err != nil {
		return err
	}
	for _, target := range targets.Targets {
		if target.ID == row.Plan.Resources.TargetID && target.Fingerprint == row.Plan.Resources.TargetFingerprint && target.Namespace == row.Plan.Resources.Namespace && target.NamespaceUID == row.Plan.Resources.NamespaceUID {
			return ctx.Err()
		}
	}
	return ErrRuntimePreparation
}
func (s *Server) runtimeResourceMutationAuthority(ctx context.Context, row storedRuntimePreparation) error {
	if err := s.runtimeBlobCurrent(ctx, row, true); err != nil {
		return err
	}
	if err := s.runtimeResourceMetadataAuthority(ctx, row, true); err != nil {
		return err
	}
	if row.Resources == nil {
		return ErrRuntimePreparation
	}
	fresh, err := s.runtimeBuildSource(ctx, row, row.Resources.Source.RunID, row.Resources.Source.CheckID)
	if err != nil || !sameRuntimeBuildSource(fresh, row.Resources.Source) {
		return ErrRuntimePreparation
	}
	for _, secret := range row.Plan.Resources.Secrets {
		if secret.ExpiresAt != "" {
			expiry, err := time.Parse(time.RFC3339Nano, secret.ExpiresAt)
			if err != nil || !time.Now().Add(5*time.Minute).Before(expiry) {
				return ErrRuntimePreparation
			}
		}
	}
	return s.runtimeResourceMetadataAuthority(ctx, row, true)
}
func (s *Server) runtimeResourceTarget(ctx context.Context, row storedRuntimePreparation, mutation bool) (deploymentkube.Target, error) {
	if err := s.runtimeResourceMetadataAuthority(ctx, row, mutation); err != nil {
		return deploymentkube.Target{}, err
	}
	target, err := s.resolveKubernetesTarget(ctx, runtimeResourceIdentity(row))
	if err != nil || target.Client == nil || target.ID != row.Plan.Resources.TargetID || target.Fingerprint != row.Plan.Resources.TargetFingerprint || target.Namespace != row.Plan.Resources.Namespace || target.NamespaceUID != row.Plan.Resources.NamespaceUID {
		return deploymentkube.Target{}, ErrRuntimePreparation
	}
	namespace, err := target.Client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
	if err != nil || namespace == nil || string(namespace.UID) != target.NamespaceUID || namespace.DeletionTimestamp != nil {
		return deploymentkube.Target{}, ErrRuntimePreparation
	}
	if err = s.runtimeResourceMetadataAuthority(ctx, row, mutation); err != nil {
		return deploymentkube.Target{}, err
	}
	return target, nil
}
func runtimeResourceSourceMatchesBuilds(row storedRuntimePreparation, source RuntimeBuildAttempt) error {
	if row.Execution != nil && !sameRuntimeBuildSource(runtimeExecutionSource(row.Execution), source) {
		return ErrRuntimePreparation
	}
	if len(row.Plan.Builds) == 0 {
		if row.Template.Source.Commit != source.Commit {
			return ErrRuntimePreparation
		}
		return nil
	}
	for _, want := range row.Plan.Builds {
		found := false
		for _, original := range row.Builds {
			if original.ServiceID == want.Selection.ServiceID && sameRuntimeBuildSource(original, source) && original.Runtime != nil && original.Runtime.Phase == "published" && original.Runtime.ImmutableImage != "" && !original.Runtime.ResourcesHeld && original.Runtime.CleanupConfirmed {
				found = true
			}
		}
		if !found {
			return ErrRuntimePreparation
		}
	}
	return nil
}
