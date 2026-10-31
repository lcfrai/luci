package product

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

func validateStoredRuntimeResources(row storedRuntimePreparation) error {
	if !validRuntimeBlobBinding(row) {
		return ErrRuntimePreparation
	}
	plan := row.Plan.Resources
	if plan == nil {
		if row.Resources != nil || len(row.PrivateSecrets) > 0 {
			return ErrRuntimePreparation
		}
		return nil
	}
	if !coolifyID.MatchString(plan.TargetID) || !hexDigest(plan.TargetFingerprint) || len(kubevalidation.IsDNS1123Label(plan.Namespace)) != 0 || !validConnectionToken(plan.NamespaceUID, 128, false) || len(plan.Volumes) > 8 || len(plan.Models) > 8 || len(plan.Secrets) > 32 || len(row.PrivateSecrets) != len(plan.Secrets) || len(plan.ImagePullServiceIDs) > 8 {
		return ErrRuntimePreparation
	}
	volumes := map[string]RuntimeVolumePlan{}
	names := map[string]bool{}
	for _, volume := range plan.Volumes {
		if volumes[volume.ServiceID].ClaimName != "" || volume.ClaimName != runtimeClaimName(row.Plan.ID, volume.ServiceID) || volume.SizeGiB < 1 || volume.SizeGiB > 1024 || volume.Purpose != "" && volume.Purpose != "telemetry-tsdb" && volume.Purpose != "telemetry-wal" || volume.Purpose != "" && volume.BindingMode != "Immediate" || len(kubevalidation.IsDNS1123Subdomain(volume.StorageClass)) != 0 || !validConnectionToken(volume.StorageClassUID, 128, false) || !validConnectionToken(volume.StorageClassVersion, 128, false) || volume.BindingMode != "Immediate" && volume.BindingMode != "WaitForFirstConsumer" || volume.ReclaimPolicy != "Delete" && volume.ReclaimPolicy != "Retain" {
			return ErrRuntimePreparation
		}
		volumes[volume.ServiceID] = volume
	}
	seen := map[string]bool{}
	var modelBytes int64
	for _, model := range plan.Models {
		volume := volumes[model.ServiceID]
		if validateRuntimeModelSource(model) != nil || seen[model.ServiceID] || volume.ClaimName == "" || volume.Purpose != "" || model.Bytes > (volume.SizeGiB<<30)-(64<<20) || !runtimeModelSafeName(plan.ModelsRelativePaths[model.ServiceID]) {
			return ErrRuntimePreparation
		}
		seen[model.ServiceID] = true
		modelBytes += model.Bytes
	}
	if modelBytes > 8<<30 || len(plan.ModelsRelativePaths) != len(plan.Models) {
		return ErrRuntimePreparation
	}
	if len(plan.Models) > 0 && (!kubeImmutableImage.MatchString(plan.ModelSinkBuilderImage) || plan.ModelSinkSourceSHA256 != runtimeModelSinkProfile()) || len(plan.Models) == 0 && (plan.ModelSinkBuilderImage != "" || plan.ModelSinkSourceSHA256 != "") {
		return ErrRuntimePreparation
	}
	for _, volume := range plan.Volumes {
		service := runtimeTemplateService(row, volume.ServiceID)
		if service == nil || !runtimeVolumeSelectionValid(*service, volume.RuntimeVolumeSelection) || volume.Purpose == "" && !seen[volume.ServiceID] {
			return ErrRuntimePreparation
		}
	}
	total := 0
	for _, secret := range plan.Secrets {
		if len(kubevalidation.IsDNS1123Subdomain(secret.Name)) != 0 || names[secret.Name] || len(secret.Keys) < 1 || len(secret.Keys) > 3 || !validConnectionToken(secret.GenerationID, 80, false) {
			return ErrRuntimePreparation
		}
		names[secret.Name] = true
		switch secret.Kind {
		case "telemetry-server", "telemetry-client":
			if len(secret.Keys) != 3 || !feedbackTimestamp(secret.ExpiresAt) {
				return ErrRuntimePreparation
			}
		case "telemetry-storage-key":
			if plan.TelemetryBlob == nil || secret.Name != plan.TelemetryBlob.SecretName || len(secret.Keys) != 1 || secret.Keys[0] != plan.TelemetryBlob.SecretKey || secret.ExpiresAt != "" {
				return ErrRuntimePreparation
			}
		case "image-pull":
			if secret.Name != plan.ImagePullSecret || len(secret.Keys) != 1 || secret.Keys[0] != ".dockerconfigjson" {
				return ErrRuntimePreparation
			}
		case "transfer-key":
			if secret.Name != runtimeTransferSecret(row.Plan.ID, secret.ServiceID) || len(secret.Keys) != 1 || secret.Keys[0] != "transfer.key" {
				return ErrRuntimePreparation
			}
		default:
			return ErrRuntimePreparation
		}
		var private *runtimePrivateSecret
		for i := range row.PrivateSecrets {
			if row.PrivateSecrets[i].Name == secret.Name {
				if private != nil {
					return ErrRuntimePreparation
				}
				private = &row.PrivateSecrets[i]
			}
		}
		if private == nil || private.GenerationID != secret.GenerationID || len(private.Data) != len(secret.Keys) || (secret.Kind == "image-pull") != (private.Type == "kubernetes.io/dockerconfigjson") || secret.Kind != "image-pull" && private.Type != "Opaque" {
			return ErrRuntimePreparation
		}
		keyNames := map[string]bool{}
		for _, key := range secret.Keys {
			data, exists := private.Data[key]
			total += len(data)
			if len(kubevalidation.IsConfigMapKey(key)) != 0 || keyNames[key] || !exists || len(data) == 0 || len(data) > 64<<10 {
				return ErrRuntimePreparation
			}
			keyNames[key] = true
		}
		if secret.Kind == "transfer-key" && !hexDigest(string(private.Data["transfer.key"])) {
			return ErrRuntimePreparation
		}
	}
	if total > 256<<10 {
		return ErrRuntimePreparation
	}
	if row.Resources == nil {
		return nil
	}
	op := row.Resources
	if row.Execution != nil && !sameRuntimeBuildSource(runtimeExecutionSource(row.Execution), op.Source) {
		return ErrRuntimePreparation
	}
	if op.Revision < 1 || op.Revision > 1_000_000_000 || !feedbackTimestamp(op.ObservedAt) || !deliverySHA.MatchString(op.Source.Commit) || !hexDigest(op.Source.SourceDigest) || !hexDigest(op.Source.RunInputDigest) || !hexDigest(op.Source.CheckDigest) || !op.Source.Attempted || !validConnectionToken(op.Source.RunID, 128, false) || !validConnectionToken(op.Source.CheckID, 128, false) || len(op.Resources) > 48 || len(op.Models) > 8 || op.HelperBuild != nil && !op.HelperBuildAttempted {
		return ErrRuntimePreparation
	}
	switch op.State {
	case "reserved", "building-helper", "pending", "ready", "unknown", "blocked":
	default:
		return ErrRuntimePreparation
	}
	if op.HelperBuild != nil && (op.HelperBuild.ProjectID != row.Plan.ProjectID || op.HelperBuild.ReleaseID != row.Plan.ID+"-model-sink" || op.HelperBuild.Commit != op.Source.Commit || op.HelperBuild.SourceDigest != runtimeModelSinkProfile() || op.HelperBuild.Target != "native-docker") {
		return ErrRuntimePreparation
	}
	keys := map[string]bool{}
	for _, resource := range op.Resources {
		key := resource.Kind + "/" + resource.Name
		if resource.Kind != "PersistentVolumeClaim" && resource.Kind != "Secret" && resource.Kind != "Pod" || keys[key] || len(kubevalidation.IsDNS1123Subdomain(resource.Name)) != 0 || !hexDigest(resource.DesiredDigest) || resource.UID != "" && !validConnectionToken(resource.UID, 128, false) || !resource.Attempted && resource.UID != "" || resource.AbsentObserved && (!resource.DeleteAttempted || resource.Kind != "Pod" || resource.UID == "") || resource.DeleteAttempted && resource.Kind != "Pod" || resource.Attempted && !op.ApplyAttempted || resource.VolumeName != "" && (resource.Kind != "PersistentVolumeClaim" || len(kubevalidation.IsDNS1123Subdomain(resource.VolumeName)) != 0) {
			return ErrRuntimePreparation
		}
		keys[key] = true
	}
	models := map[string]bool{}
	for _, model := range op.Models {
		if !seen[model.ServiceID] || models[model.ServiceID] || model.OperationID != row.Plan.ID+"-model-"+reviewDigest(model.ServiceID)[:12] || model.PodName != runtimeTransferPod(row.Plan.ID, model.ServiceID) || model.UploadAttempted && (!op.ApplyAttempted || model.PodUID == "" || model.PVCUID == "" || !runtimeContainerIdentity(model.ContainerID) || !runtimeContainerIdentity(model.ContainerImageID)) || model.Download != nil && !model.UploadAttempted || model.Receipt != nil && !model.UploadAttempted {
			return ErrRuntimePreparation
		}
		models[model.ServiceID] = true
		var expected RuntimeModelSource
		for _, source := range plan.Models {
			if source.ServiceID == model.ServiceID {
				expected = source
			}
		}
		if model.Download != nil && (model.Download.Bytes != expected.Bytes || model.Download.SHA256 != expected.SHA256 || !feedbackTimestamp(model.Download.ObservedAt)) {
			return ErrRuntimePreparation
		}
		if model.Receipt != nil {
			receipt := model.Receipt
			if receipt.Protocol != 1 || receipt.PreparationID != row.Plan.ID || receipt.OperationID != model.OperationID || receipt.PVCUID != model.PVCUID || receipt.Bytes != expected.Bytes || receipt.SHA256 != expected.SHA256 || receipt.Path != plan.ModelsRelativePaths[model.ServiceID] || receipt.State != "stored-reported" {
				return ErrRuntimePreparation
			}
		}
		if model.OriginalZeroExit && (model.Download == nil || model.Receipt == nil || !model.UploadAttempted || model.ContainerImageID == "") {
			return ErrRuntimePreparation
		}
	}
	if op.State == "ready" {
		if !op.ApplyAttempted || len(op.Resources) != len(plan.Volumes)+len(plan.Secrets)+len(plan.Models) || len(op.Models) != len(plan.Models) {
			return ErrRuntimePreparation
		}
		for _, volume := range plan.Volumes {
			r := runtimeOwnedResource(op, "PersistentVolumeClaim", volume.ClaimName)
			if r == nil || r.UID == "" || r.VolumeName == "" || r.State != "bound" {
				return ErrRuntimePreparation
			}
		}
		for _, secret := range plan.Secrets {
			r := runtimeOwnedResource(op, "Secret", secret.Name)
			if r == nil || r.UID == "" {
				return ErrRuntimePreparation
			}
		}
		for _, model := range op.Models {
			r := runtimeOwnedResource(op, "Pod", model.PodName)
			if !model.OriginalZeroExit || r == nil || !r.DeleteAttempted || !r.AbsentObserved {
				return ErrRuntimePreparation
			}
		}
	}
	return nil
}
func cloneRuntimeResourceOperation(op *RuntimeResourceOperation) (*RuntimeResourceOperation, error) {
	if op == nil {
		return nil, nil
	}
	raw, err := json.Marshal(op)
	if err != nil {
		return nil, err
	}
	var copy RuntimeResourceOperation
	if json.Unmarshal(raw, &copy) != nil {
		return nil, ErrRuntimePreparation
	}
	return &copy, nil
}
func runtimeResourceContinues(before, after *RuntimeResourceOperation) bool {
	if before == nil {
		return after != nil
	}
	if after == nil || !sameRuntimeBuildSource(before.Source, after.Source) || before.ApplyAttempted && !after.ApplyAttempted || before.HelperBuildAttempted && !after.HelperBuildAttempted || before.HelperBuild != nil && (after.HelperBuild == nil || !previewruntime.Continues(before.HelperBuild.Identity, after.HelperBuild.Identity) || before.HelperBuild.ImmutableImage != "" && before.HelperBuild.ImmutableImage != after.HelperBuild.ImmutableImage) {
		return false
	}
	for _, old := range before.Resources {
		found := false
		for _, next := range after.Resources {
			if old.Kind == next.Kind && old.Name == next.Name {
				found = true
				if old.DesiredDigest != next.DesiredDigest || old.Attempted && !next.Attempted || old.UID != "" && old.UID != next.UID || old.VolumeName != "" && old.VolumeName != next.VolumeName || old.DeleteAttempted && !next.DeleteAttempted || old.AbsentObserved && !next.AbsentObserved {
					return false
				}
			}
		}
		if !found {
			return false
		}
	}
	for _, old := range before.Models {
		found := false
		for _, next := range after.Models {
			if old.ServiceID == next.ServiceID {
				found = true
				if old.OperationID != next.OperationID || old.PodName != next.PodName || old.PodUID != "" && old.PodUID != next.PodUID || old.PVCUID != "" && old.PVCUID != next.PVCUID || old.ContainerID != "" && old.ContainerID != next.ContainerID || old.ContainerImageID != "" && old.ContainerImageID != next.ContainerImageID || old.UploadAttempted && !next.UploadAttempted || old.OriginalZeroExit && !next.OriginalZeroExit || old.Download != nil && reviewDigest(old.Download) != reviewDigest(next.Download) || old.Receipt != nil && reviewDigest(old.Receipt) != reviewDigest(next.Receipt) {
					return false
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func (s *Server) persistRuntimeResource(ctx context.Context, before storedRuntimePreparation, next *RuntimeResourceOperation) (storedRuntimePreparation, error) {
	s.runtimePreparationMu.Lock()
	defer s.runtimePreparationMu.Unlock()
	file, current, err := s.readRuntimePreparation(before.Plan.Actor, before.Plan.ID)
	if err != nil {
		return before, err
	}
	if current.Revision != before.Revision || reviewDigest(current) != reviewDigest(before) || !runtimeResourceContinues(current.Resources, next) {
		return before, ErrRuntimePreparation
	}
	current.Resources = next
	current.Revision++
	if err = validateStoredRuntimeResources(current); err != nil {
		return before, err
	}
	if err = runtimePreparationBytes(current); err != nil {
		return before, err
	}
	// Receipt retention may finish after a provider response and revoked access.
	// It authorizes no new RPC and never exposes private values through responses.
	if err = durableRunJSON(file, current); err != nil {
		return before, err
	}
	return current, nil
}

// The HTTP transport may consume the model reader on its own goroutine. This
// one writer serializes original receipt CAS; no record mutation races a read.
type runtimeResourceWriter struct {
	mu       sync.Mutex
	server   *Server
	row      storedRuntimePreparation
	readOnly bool
}

func (w *runtimeResourceWriter) snapshot() storedRuntimePreparation {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.row
}
func (w *runtimeResourceWriter) mutate(ctx context.Context, change func(*RuntimeResourceOperation) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	copy, err := cloneRuntimeResourceOperation(w.row.Resources)
	if err != nil || copy == nil {
		return ErrRuntimePreparation
	}
	if err = change(copy); err != nil {
		return err
	}
	copy.Revision++
	copy.ObservedAt = coordinatorPreviewNow()
	if w.readOnly {
		if !runtimeResourceContinues(w.row.Resources, copy) {
			return ErrRuntimePreparation
		}
		w.row.Resources = copy
		return nil
	}
	updated, err := w.server.persistRuntimeResource(ctx, w.row, copy)
	if err == nil {
		w.row = updated
	}
	return err
}
func runtimeOwnedResource(op *RuntimeResourceOperation, kind, name string) *RuntimeOwnedResource {
	for i := range op.Resources {
		if op.Resources[i].Kind == kind && op.Resources[i].Name == name {
			return &op.Resources[i]
		}
	}
	return nil
}
func runtimeModelTransfer(op *RuntimeResourceOperation, service string) *RuntimeModelTransfer {
	for i := range op.Models {
		if op.Models[i].ServiceID == service {
			return &op.Models[i]
		}
	}
	return nil
}
func runtimePrivateSecretFor(row storedRuntimePreparation, name string) *runtimePrivateSecret {
	for i := range row.PrivateSecrets {
		if row.PrivateSecrets[i].Name == name {
			return &row.PrivateSecrets[i]
		}
	}
	return nil
}
func runtimeTransferKey(row storedRuntimePreparation, service string) (string, error) {
	secret := runtimePrivateSecretFor(row, runtimeTransferSecret(row.Plan.ID, service))
	if secret == nil {
		return "", ErrRuntimePreparation
	}
	key := string(secret.Data["transfer.key"])
	if !hexDigest(key) {
		return "", ErrRuntimePreparation
	}
	return key, nil
}
func runtimeHelperImage(row storedRuntimePreparation) (string, error) {
	if row.Resources == nil || row.Resources.HelperBuild == nil {
		return "", ErrRuntimePreparation
	}
	out := row.Resources.HelperBuild
	if out.Phase != "published" || out.ResourcesHeld || !out.CleanupConfirmed || !strings.HasPrefix(out.ImmutableImage, row.Plan.Registry.Repository+"@sha256:") || !kubeImmutableImage.MatchString(out.ImmutableImage) {
		return "", ErrRuntimePreparation
	}
	return out.ImmutableImage, nil
}
