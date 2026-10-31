package product

import (
	"context"
	"errors"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func runtimeContainerIdentity(value string) bool {
	return len(value) > 0 && len(value) <= 512 && !strings.ContainsAny(value, "\r\n\x00")
}
func runtimeSinkStatus(pod *corev1.Pod) (*corev1.ContainerStatus, error) {
	if pod == nil || len(pod.Status.ContainerStatuses) != 1 || len(pod.Status.InitContainerStatuses) != 0 || len(pod.Status.EphemeralContainerStatuses) != 0 {
		return nil, errRuntimeResourcePending
	}
	status := &pod.Status.ContainerStatuses[0]
	if status.Name != "model-sink" || status.RestartCount != 0 || status.LastTerminationState.Terminated != nil {
		return nil, ErrRuntimePreparation
	}
	return status, nil
}
func (s *Server) runtimeOriginalPod(ctx context.Context, w *runtimeResourceWriter, target deploymentkube.Target, model RuntimeModelSource) (*corev1.Pod, error) {
	row := w.snapshot()
	image, err := runtimeHelperImage(row)
	if err != nil {
		return nil, err
	}
	state := runtimeModelTransfer(row.Resources, model.ServiceID)
	if state == nil || state.PodUID == "" {
		return nil, ErrRuntimePreparation
	}
	desired, err := runtimeDesiredTransferPod(row, model, state.PVCUID, image)
	if err != nil {
		return nil, err
	}
	original := runtimeOwnedResource(row.Resources, "Pod", state.PodName)
	if original == nil || original.UID != state.PodUID {
		return nil, ErrRuntimePreparation
	}
	var got corev1.Pod
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "pods", state.PodName, nil, &got); err != nil {
		return nil, err
	}
	if (!runtimeResourceStampMatches(row, &got.ObjectMeta, state.PodName, original.DesiredDigest, state.PodUID) || got.DeletionTimestamp != nil && !original.DeleteAttempted) || !deploymentkube.PreparationPodSpecMatches(desired.Spec, got.Spec) {
		return nil, ErrRuntimePreparation
	}
	if err = s.runtimeResourceMetadataAuthority(ctx, row, false); err != nil {
		return nil, err
	}
	return &got, nil
}
func (s *Server) runtimeTransferClaim(ctx context.Context, w *runtimeResourceWriter, target deploymentkube.Target, model RuntimeModelSource, bound bool) (corev1.PersistentVolumeClaim, error) {
	row := w.snapshot()
	var volume RuntimeVolumePlan
	for _, v := range row.Plan.Resources.Volumes {
		if v.ServiceID == model.ServiceID {
			volume = v
		}
	}
	var got corev1.PersistentVolumeClaim
	if volume.ClaimName == "" {
		return got, ErrRuntimePreparation
	}
	if err := deploymentkube.PreparationResourceRPC(ctx, target, "GET", "persistentvolumeclaims", volume.ClaimName, nil, &got); err != nil {
		return got, err
	}
	if !runtimeClaimSpecOK(row, volume, &got) {
		return got, ErrRuntimePreparation
	}
	if bound {
		requested := runtimeDesiredPVC(row, volume).Spec.Resources.Requests[corev1.ResourceStorage]
		if got.Status.Phase != corev1.ClaimBound || got.Spec.VolumeName == "" || got.Status.Capacity.Storage().Cmp(requested) < 0 {
			return got, errRuntimeResourcePending
		}
		if err := w.mutate(ctx, func(op *RuntimeResourceOperation) error {
			original := runtimeOwnedResource(op, "PersistentVolumeClaim", volume.ClaimName)
			if original.VolumeName != "" && original.VolumeName != got.Spec.VolumeName {
				return ErrRuntimePreparation
			}
			original.VolumeName = got.Spec.VolumeName
			original.State = "bound"
			return nil
		}); err != nil {
			return got, err
		}
	}
	return got, s.runtimeResourceMetadataAuthority(ctx, w.snapshot(), false)
}
func (s *Server) runRuntimeModelTransfer(ctx context.Context, w *runtimeResourceWriter, model RuntimeModelSource) error {
	target, err := s.runtimeResourceWriteTarget(ctx, w)
	if err != nil {
		return err
	}
	claim, err := s.runtimeTransferClaim(ctx, w, target, model, false)
	if err != nil {
		return err
	}
	image, err := runtimeHelperImage(w.snapshot())
	if err != nil {
		return err
	}
	if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		state := runtimeModelTransfer(op, model.ServiceID)
		if state == nil || state.PVCUID != "" {
			return ErrRuntimePreparation
		}
		state.PVCUID = string(claim.UID)
		op.Resources = append(op.Resources, RuntimeOwnedResource{Kind: "Pod", Name: state.PodName, DesiredDigest: runtimeTransferPodDigest(w.row, model.ServiceID, image), State: "reserved"})
		return nil
	}); err != nil {
		return err
	}
	row := w.snapshot()
	state := *runtimeModelTransfer(row.Resources, model.ServiceID)
	desired, err := runtimeDesiredTransferPod(row, model, state.PVCUID, image)
	if err != nil {
		return err
	}
	var existing corev1.Pod
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "pods", state.PodName, nil, &existing); !errors.Is(err, deploymentkube.ErrPreparationAbsent) {
		return ErrRuntimePreparation
	}
	if err = markRuntimeObjectAttempt(ctx, w, "Pod", state.PodName); err != nil {
		return err
	}
	target, err = s.runtimeResourceWriteTarget(ctx, w)
	if err != nil {
		return err
	}
	if _, err = s.runtimeTransferClaim(ctx, w, target, model, false); err != nil {
		return err
	}
	var created corev1.Pod
	if err = s.runtimeResourceMutationAuthority(ctx, w.snapshot()); err != nil {
		return err
	}
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "POST", "pods", "", desired, &created); err != nil {
		return err
	}
	if err = retainRuntimeObjectUID(ctx, w, "Pod", state.PodName, string(created.UID), created.ResourceVersion); err != nil {
		return err
	}
	if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		next := runtimeModelTransfer(op, model.ServiceID)
		next.PodUID = string(created.UID)
		return nil
	}); err != nil {
		return err
	}
	// The Pod itself is a genuine WFFC first consumer. No synthetic Bound flag,
	// namespace creation or replacement of an existing volume is used.
	for {
		target, err = s.runtimeResourceTarget(ctx, w.snapshot(), false)
		if err != nil {
			return err
		}
		pod, podErr := s.runtimeOriginalPod(ctx, w, target, model)
		if podErr != nil {
			return podErr
		}
		status, statusErr := runtimeSinkStatus(pod)
		if statusErr != nil && !errors.Is(statusErr, errRuntimeResourcePending) {
			return statusErr
		}
		if statusErr == nil && status.State.Terminated != nil {
			return ErrRuntimePreparation
		}
		_, claimErr := s.runtimeTransferClaim(ctx, w, target, model, true)
		if claimErr != nil && !errors.Is(claimErr, errRuntimeResourcePending) {
			return claimErr
		}
		if statusErr == nil && claimErr == nil && status.Ready && status.State.Running != nil && runtimeContainerIdentity(status.ContainerID) && runtimeContainerIdentity(status.ImageID) {
			if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
				next := runtimeModelTransfer(op, model.ServiceID)
				next.ContainerID = status.ContainerID
				next.ContainerImageID = status.ImageID
				next.State = "ready-for-original-upload"
				return nil
			}); err != nil {
				return err
			}
			break
		}
		if err = runtimePreparationPause(ctx); err != nil {
			return err
		}
	}
	if err = s.runtimeResourceMutationAuthority(ctx, w.snapshot()); err != nil {
		return err
	}
	if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		next := runtimeModelTransfer(op, model.ServiceID)
		if next.UploadAttempted {
			return ErrRuntimePreparation
		}
		next.UploadAttempted = true
		next.State = "unknown"
		return nil
	}); err != nil {
		return err
	}
	authorize := func(call context.Context) error { return s.runtimeResourceMetadataAuthority(call, w.snapshot(), true) }
	record := func(call context.Context, proof RuntimeModelDownloadProof) error {
		return w.mutate(call, func(op *RuntimeResourceOperation) error {
			next := runtimeModelTransfer(op, model.ServiceID)
			if !next.UploadAttempted || next.Download != nil {
				return ErrRuntimePreparation
			}
			next.Download = &proof
			return nil
		})
	}
	source, err := openRuntimeModel(ctx, model, authorize, record, nil)
	if err != nil {
		return err
	}
	defer source.Close()
	key, err := runtimeTransferKey(w.snapshot(), model.ServiceID)
	if err != nil {
		return err
	}
	before := func(call context.Context) error {
		if err := s.runtimeResourceMutationAuthority(call, w.snapshot()); err != nil {
			return err
		}
		current, err := s.runtimeResourceTarget(call, w.snapshot(), true)
		if err != nil {
			return err
		}
		if _, err = s.runtimeTransferClaim(call, w, current, model, true); err != nil {
			return err
		}
		pod, err := s.runtimeOriginalPod(call, w, current, model)
		if err != nil {
			return err
		}
		got, err := runtimeSinkStatus(pod)
		if err != nil {
			return err
		}
		original := runtimeModelTransfer(w.snapshot().Resources, model.ServiceID)
		if got.ContainerID != original.ContainerID || got.ImageID != original.ContainerImageID || got.RestartCount != 0 {
			return ErrRuntimePreparation
		}
		return s.runtimeResourceMutationAuthority(call, w.snapshot())
	}
	target, err = s.runtimeResourceTarget(ctx, w.snapshot(), true)
	if err != nil {
		return err
	}
	receipt, uploadErr := deploymentkube.UploadOriginalModel(ctx, target, state.PodName, key, source, before)
	closeErr := source.Close()
	if uploadErr == nil && closeErr == nil {
		if !runtimeReceiptMatches(receipt, w.snapshot(), model, *runtimeModelTransfer(w.snapshot().Resources, model.ServiceID)) {
			return ErrRuntimePreparation
		}
		if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
			next := runtimeModelTransfer(op, model.ServiceID)
			copy := receipt
			next.Receipt = &copy
			return nil
		}); err != nil {
			return err
		}
	}
	// Even a lost HTTP response is only observable using the same original Pod
	// zero-exit fact + bounded termination receipt AND controller complete-body
	// proof. There is never a second POST or file adoption by name/hash alone.
	for {
		target, err = s.runtimeResourceTarget(ctx, w.snapshot(), false)
		if err != nil {
			return err
		}
		err = s.observeRuntimeModel(ctx, w, target, model)
		if err == nil {
			break
		}
		observed := runtimeModelTransfer(w.snapshot().Resources, model.ServiceID)
		if errors.Is(err, errRuntimeResourcePending) && observed != nil && observed.OriginalZeroExit && observed.Receipt != nil {
			break
		}
		if !errors.Is(err, errRuntimeResourcePending) {
			return err
		}
		if uploadErr != nil || closeErr != nil {
			return errRuntimeResourcePending
		}
		if err = runtimePreparationPause(ctx); err != nil {
			return err
		}
	}
	return s.deleteRuntimeTransferPod(ctx, w, model)
}

func (s *Server) observeRuntimeModel(ctx context.Context, w *runtimeResourceWriter, target deploymentkube.Target, model RuntimeModelSource) error {
	row := w.snapshot()
	state := runtimeModelTransfer(row.Resources, model.ServiceID)
	if state == nil || state.PodUID == "" || !state.UploadAttempted || state.Download == nil {
		return ErrRuntimePreparation
	}
	pod, err := s.runtimeOriginalPod(ctx, w, target, model)
	if errors.Is(err, deploymentkube.ErrPreparationAbsent) {
		original := runtimeOwnedResource(row.Resources, "Pod", state.PodName)
		if original == nil || !original.DeleteAttempted || !state.OriginalZeroExit || state.Receipt == nil {
			return ErrRuntimePreparation
		}
		return w.mutate(ctx, func(op *RuntimeResourceOperation) error {
			r := runtimeOwnedResource(op, "Pod", state.PodName)
			r.AbsentObserved = true
			r.State = "absent"
			runtimeModelTransfer(op, model.ServiceID).State = "stored-original-helper-retired"
			return nil
		})
	}
	if err != nil {
		return err
	}
	status, err := runtimeSinkStatus(pod)
	if err != nil {
		return err
	}
	if status.ContainerID != state.ContainerID || status.ImageID != state.ContainerImageID {
		return ErrRuntimePreparation
	}
	terminated := status.State.Terminated
	if terminated == nil {
		return errRuntimeResourcePending
	}
	if terminated.ExitCode != 0 || terminated.Signal != 0 || terminated.Reason != "Completed" || terminated.ContainerID != state.ContainerID || terminated.StartedAt.IsZero() || terminated.FinishedAt.IsZero() || terminated.FinishedAt.Before(&terminated.StartedAt) {
		return ErrRuntimePreparation
	}
	receipt, err := deploymentkube.DecodeModelSinkReceipt([]byte(terminated.Message))
	if err != nil || !runtimeReceiptMatches(receipt, row, model, *state) {
		return ErrRuntimePreparation
	}
	if state.Receipt != nil && reviewDigest(state.Receipt) != reviewDigest(receipt) {
		return ErrRuntimePreparation
	}
	if _, err = s.runtimeTransferClaim(ctx, w, target, model, true); err != nil {
		return err
	}
	if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		next := runtimeModelTransfer(op, model.ServiceID)
		copy := receipt
		next.Receipt = &copy
		next.OriginalZeroExit = true
		next.State = "stored-original-zero-cleanup-pending"
		return nil
	}); err != nil {
		return err
	}
	return errRuntimeResourcePending
}

func (s *Server) deleteRuntimeTransferPod(ctx context.Context, w *runtimeResourceWriter, model RuntimeModelSource) error {
	row := w.snapshot()
	state := runtimeModelTransfer(row.Resources, model.ServiceID)
	if state == nil || !state.OriginalZeroExit || state.Receipt == nil || state.Download == nil {
		return ErrRuntimePreparation
	}
	// Cleanup does not allocate or require current code/registry credentials.
	target, err := s.runtimeResourceTarget(ctx, row, false)
	if err != nil {
		return err
	}
	if _, err = s.runtimeOriginalPod(ctx, w, target, model); err != nil {
		return err
	}
	if err = w.mutate(ctx, func(op *RuntimeResourceOperation) error {
		r := runtimeOwnedResource(op, "Pod", state.PodName)
		if r.DeleteAttempted {
			return ErrRuntimePreparation
		}
		r.DeleteAttempted = true
		return nil
	}); err != nil {
		return err
	}
	target, err = s.runtimeResourceTarget(ctx, w.snapshot(), false)
	if err != nil {
		return err
	}
	if _, err = s.runtimeOriginalPod(ctx, w, target, model); err != nil {
		return err
	}
	uid := types.UID(state.PodUID)
	policy := metav1.DeletePropagationBackground
	options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}, PropagationPolicy: &policy}
	// DELETE has a UID precondition; attempted intent is retained even on lost
	// response. A repeated Observe only confirms absence, never deletes again.
	if err = deploymentkube.PreparationResourceRPC(ctx, target, "DELETE", "pods", state.PodName, options, nil); err != nil {
		return err
	}
	for {
		err = s.observeRuntimeModel(ctx, w, target, model)
		if err == nil {
			r := runtimeOwnedResource(w.snapshot().Resources, "Pod", state.PodName)
			if r.AbsentObserved {
				return nil
			}
		} else if !errors.Is(err, errRuntimeResourcePending) {
			return err
		}
		if err = runtimePreparationPause(ctx); err != nil {
			return err
		}
		target, err = s.runtimeResourceTarget(ctx, w.snapshot(), false)
		if err != nil {
			return err
		}
	}
}
