package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Every remote creation has an original durable attempted marker. Only this
// caller that wrote it may issue one POST; GET never substitutes a same-name
// object for a lost positive Create UID acknowledgement.
func (s *Server) runRuntimeResourceWrites(ctx context.Context, w *runtimeResourceWriter) error {
	if err := s.runtimeResourceMutationAuthority(ctx, w.snapshot()); err != nil {
		return err
	}
	if err := w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		if op.ApplyAttempted {
			return ErrRuntimePreparation
		}
		op.ApplyAttempted = true
		op.State = "pending"
		return nil
	}); err != nil {
		return err
	}
	row := w.snapshot()
	for _, volume := range row.Plan.Resources.Volumes {
		if err := s.createRuntimeClaim(ctx, w, volume); err != nil {
			return err
		}
	}
	for _, secret := range row.Plan.Resources.Secrets {
		if err := s.createRuntimeSecret(ctx, w, secret); err != nil {
			return err
		}
	}
	for _, model := range row.Plan.Resources.Models {
		if err := s.runRuntimeModelTransfer(ctx, w, model); err != nil {
			return err
		}
	}
	// Immediate telemetry volumes must reach Bound before deployment review.
	// Model WFFC claims have their original helper as a real first consumer.
	for {
		err := s.observeRuntimeResourceObjects(ctx, w)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errRuntimeResourcePending) {
			return err
		}
		if err = runtimePreparationPause(ctx); err != nil {
			return err
		}
	}
}

func (s *Server) runtimeResourceWriteTarget(ctx context.Context, w *runtimeResourceWriter) (deploymentkube.Target, error) {
	row := w.snapshot()
	if err := s.runtimeResourceMutationAuthority(ctx, row); err != nil {
		return deploymentkube.Target{}, err
	}
	return s.runtimeResourceTarget(ctx, row, true)
}
func markRuntimeObjectAttempt(ctx context.Context, w *runtimeResourceWriter, kind, name string) error {
	return w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		r := runtimeOwnedResource(op, kind, name)
		if r == nil || r.Attempted {
			return ErrRuntimePreparation
		}
		r.Attempted = true
		r.State = "unknown"
		return nil
	})
}
func retainRuntimeObjectUID(ctx context.Context, w *runtimeResourceWriter, kind, name, uid, version string) error {
	if !validConnectionToken(uid, 128, false) || !validConnectionToken(version, 128, false) {
		return ErrRuntimePreparation
	}
	record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return w.mutate(record, func(op *RuntimeResourceOperation) error {
		r := runtimeOwnedResource(op, kind, name)
		if r == nil || !r.Attempted || r.UID != "" && r.UID != uid {
			return ErrRuntimePreparation
		}
		r.UID, r.ResourceVersion, r.State = uid, version, "observed"
		return nil
	})
}
func runtimeClaimSpecOK(row storedRuntimePreparation, volume RuntimeVolumePlan, got *corev1.PersistentVolumeClaim) bool {
	owned := runtimeOwnedResource(row.Resources, "PersistentVolumeClaim", volume.ClaimName)
	return got != nil && owned != nil && owned.UID != "" && runtimeResourceOwned(row, &got.ObjectMeta, volume.ClaimName, owned.DesiredDigest, owned.UID) && deploymentkube.PreparationPVCSpecMatches(runtimeDesiredPVC(row, volume).Spec, got.Spec) && (owned.VolumeName == "" || owned.VolumeName == got.Spec.VolumeName)
}
func (s *Server) createRuntimeClaim(ctx context.Context, w *runtimeResourceWriter, volume RuntimeVolumePlan) error {
	target, err := s.runtimeResourceWriteTarget(ctx, w)
	if err != nil {
		return err
	}
	storage, err := target.Client.StorageV1().StorageClasses().Get(ctx, volume.StorageClass, metav1.GetOptions{})
	if err != nil || storage == nil || string(storage.UID) != volume.StorageClassUID || storage.ResourceVersion != volume.StorageClassVersion || storage.DeletionTimestamp != nil {
		return ErrRuntimePreparation
	}
	var existing corev1.PersistentVolumeClaim
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "persistentvolumeclaims", volume.ClaimName, nil, &existing); !errors.Is(err, deploymentkube.ErrPreparationAbsent) {
		return ErrRuntimePreparation
	}
	if err = markRuntimeObjectAttempt(ctx, w, "PersistentVolumeClaim", volume.ClaimName); err != nil {
		return err
	}
	target, err = s.runtimeResourceWriteTarget(ctx, w)
	if err != nil {
		return err
	}
	// Storage class is re-read after the durable-marker wait as well.
	storage, err = target.Client.StorageV1().StorageClasses().Get(ctx, volume.StorageClass, metav1.GetOptions{})
	if err != nil || storage == nil || string(storage.UID) != volume.StorageClassUID || storage.ResourceVersion != volume.StorageClassVersion || storage.DeletionTimestamp != nil {
		return ErrRuntimePreparation
	}
	if err = s.runtimeResourceMutationAuthority(ctx, w.snapshot()); err != nil {
		return err
	}
	var created corev1.PersistentVolumeClaim
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "POST", "persistentvolumeclaims", "", runtimeDesiredPVC(w.snapshot(), volume), &created); err != nil {
		return err
	}
	// Retain the actual acknowledged UID even if admission altered its spec.
	if err = retainRuntimeObjectUID(ctx, w, "PersistentVolumeClaim", volume.ClaimName, string(created.UID), created.ResourceVersion); err != nil {
		return err
	}
	if !runtimeClaimSpecOK(w.snapshot(), volume, &created) {
		return ErrRuntimePreparation
	}
	return s.runtimeResourceMetadataAuthority(ctx, w.snapshot(), true)
}
func (s *Server) createRuntimeSecret(ctx context.Context, w *runtimeResourceWriter, plan RuntimeSecretPlan) error {
	target, err := s.runtimeResourceWriteTarget(ctx, w)
	if err != nil {
		return err
	}
	desired, err := runtimeDesiredSecret(w.snapshot(), plan)
	if err != nil {
		return err
	}
	var existing corev1.Secret
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "secrets", plan.Name, nil, &existing); !errors.Is(err, deploymentkube.ErrPreparationAbsent) {
		return ErrRuntimePreparation
	}
	if err = markRuntimeObjectAttempt(ctx, w, "Secret", plan.Name); err != nil {
		return err
	}
	target, err = s.runtimeResourceWriteTarget(ctx, w)
	if err != nil {
		return err
	}
	var created corev1.Secret
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "POST", "secrets", "", desired, &created); err != nil {
		return err
	}
	if err = retainRuntimeObjectUID(ctx, w, "Secret", plan.Name, string(created.UID), created.ResourceVersion); err != nil {
		return err
	}
	if !runtimeSecretMatches(w.snapshot(), plan, &created) {
		return ErrRuntimePreparation
	}
	return s.runtimeResourceMetadataAuthority(ctx, w.snapshot(), true)
}
func runtimeSecretMatches(row storedRuntimePreparation, plan RuntimeSecretPlan, got *corev1.Secret) bool {
	original := runtimeOwnedResource(row.Resources, "Secret", plan.Name)
	desired, err := runtimeDesiredSecret(row, plan)
	return err == nil && got != nil && original != nil && original.UID != "" && runtimeResourceOwned(row, &got.ObjectMeta, plan.Name, original.DesiredDigest, original.UID) && got.Immutable != nil && *got.Immutable && got.Type == desired.Type && len(got.StringData) == 0 && equality.Semantic.DeepEqual(desired.Data, got.Data)
}

func (s *Server) observeRuntimeResourceObjects(ctx context.Context, w *runtimeResourceWriter) error {
	row := w.snapshot()
	target, err := s.runtimeResourceTarget(ctx, row, false)
	if err != nil {
		return err
	}
	pending := false
	for _, volume := range row.Plan.Resources.Volumes {
		original := runtimeOwnedResource(row.Resources, "PersistentVolumeClaim", volume.ClaimName)
		if original == nil || original.UID == "" {
			return ErrRuntimePreparation
		}
		var got corev1.PersistentVolumeClaim
		if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "persistentvolumeclaims", volume.ClaimName, nil, &got); err != nil {
			return err
		}
		if !runtimeClaimSpecOK(row, volume, &got) {
			return ErrRuntimePreparation
		}
		bound := got.Status.Phase == corev1.ClaimBound && got.Spec.VolumeName != "" && got.Status.Capacity.Storage().Cmp(runtimeDesiredPVC(row, volume).Spec.Resources.Requests[corev1.ResourceStorage]) >= 0
		if !bound {
			pending = true
		} else if original.VolumeName == "" {
			if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
				r := runtimeOwnedResource(op, "PersistentVolumeClaim", volume.ClaimName)
				r.VolumeName = got.Spec.VolumeName
				r.State = "bound"
				return nil
			}); err != nil {
				return err
			}
			row = w.snapshot()
		}
		if err = s.runtimeResourceMetadataAuthority(ctx, row, false); err != nil {
			return err
		}
	}
	for _, plan := range row.Plan.Resources.Secrets {
		var got corev1.Secret
		if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "secrets", plan.Name, nil, &got); err != nil {
			return err
		}
		if !runtimeSecretMatches(row, plan, &got) {
			return ErrRuntimePreparation
		}
		if err = s.runtimeResourceMetadataAuthority(ctx, row, false); err != nil {
			return err
		}
	}
	for _, model := range row.Plan.Resources.Models {
		if err = s.observeRuntimeModel(ctx, w, target, model); err != nil {
			if errors.Is(err, errRuntimeResourcePending) {
				pending = true
			} else {
				return err
			}
		}
	}
	if _, err = s.runtimeResourceTarget(ctx, w.snapshot(), false); err != nil {
		return err
	}
	if pending {
		return errRuntimeResourcePending
	}
	return w.mutate(ctx, func(op *RuntimeResourceOperation) error { op.State = "ready"; return nil })
}
