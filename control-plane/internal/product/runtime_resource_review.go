package product

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/distribution/reference"
	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

func runtimeClaimName(preparationID, serviceID string) string {
	return "luci-model-" + preparationID[5:13] + "-" + reviewDigest(serviceID)[:12]
}
func runtimeTransferSecret(preparationID, serviceID string) string {
	return "luci-transfer-" + preparationID[5:13] + "-" + reviewDigest(serviceID)[:12]
}
func runtimeTransferPod(preparationID, serviceID string) string {
	return "luci-transfer-" + preparationID[5:13] + "-" + reviewDigest(serviceID)[:12]
}
func runtimeImagePullSecret(preparationID string) string { return "luci-images-" + preparationID[5:21] }
func runtimeResourceIdentity(row storedRuntimePreparation) deploymentkube.Identity {
	return deploymentkube.Identity{ProjectID: row.Plan.ProjectID, ActorID: row.Plan.Actor.UserID, TeamID: row.Plan.Actor.TeamID, TargetID: row.Plan.Resources.TargetID, TargetFingerprint: row.Plan.Resources.TargetFingerprint, OperationID: row.Plan.ID + "-resources", PlanDigest: row.Plan.Digest}
}
func runtimeTemplateService(row storedRuntimePreparation, id string) *deploymentplan.Service {
	for i := range row.Template.Services {
		if row.Template.Services[i].ID == id {
			return &row.Template.Services[i]
		}
	}
	return nil
}

func (s *Server) reviewRuntimeResourcePlan(ctx context.Context, actor management.ExecutionActorRef, row storedRuntimePreparation, input RuntimeResourceSelection) (*RuntimeResourcePlan, []runtimePrivateSecret, error) {
	if !input.AcknowledgeNewResources || len(input.Volumes) > 8 || len(input.Models) > 8 || len(input.ImagePullServiceIDs) > 8 {
		return nil, nil, ErrRuntimePreparation
	}
	if len(row.Template.Infrastructure.Namespace) == 0 || row.Template.Infrastructure.Reference == "" || row.Template.Infrastructure.Kind != "kubernetes" {
		return nil, nil, errors.New("Build images first, then connect the namespace before reviewing runtime resources")
	}
	if err := s.kubernetesAuthority(ctx, actor, true, true, row.Plan.MonthlyBudgetUSD); err != nil {
		return nil, nil, err
	}
	ctx = WithKubernetesTargetResolution(ctx)
	_, targets, err := s.readKubernetesTargets(actor.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	var descriptor *KubernetesTargetDescriptor
	for i := range targets.Targets {
		if targets.Targets[i].ID == row.Template.Infrastructure.Reference && targets.Targets[i].Namespace == row.Template.Infrastructure.Namespace {
			copy := targets.Targets[i].KubernetesTargetDescriptor
			descriptor = &copy
		}
	}
	if descriptor == nil {
		return nil, nil, ErrRuntimePreparation
	}
	identity := deploymentkube.Identity{ProjectID: actor.ProjectID, ActorID: actor.UserID, TeamID: actor.TeamID, TargetID: descriptor.ID, TargetFingerprint: descriptor.Fingerprint, OperationID: row.Plan.ID + "-resources", PlanDigest: reviewDigest(input)}
	target, err := s.resolveKubernetesTarget(ctx, identity)
	if err != nil {
		return nil, nil, err
	}
	namespace, err := target.Client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
	if err != nil || namespace == nil || string(namespace.UID) != target.NamespaceUID || namespace.DeletionTimestamp != nil {
		return nil, nil, ErrRuntimePreparation
	}
	plan := &RuntimeResourcePlan{TargetID: target.ID, TargetFingerprint: target.Fingerprint, Namespace: target.Namespace, NamespaceUID: target.NamespaceUID, Volumes: []RuntimeVolumePlan{}, Models: append([]RuntimeModelSource{}, input.Models...), Secrets: []RuntimeSecretPlan{}, ModelsRelativePaths: map[string]string{}}
	private := []runtimePrivateSecret{}
	volumes := map[string]RuntimeVolumePlan{}
	for _, selected := range input.Volumes {
		service := runtimeTemplateService(row, selected.ServiceID)
		if service == nil || !runtimeVolumeSelectionValid(*service, selected) || volumes[selected.ServiceID].ClaimName != "" || selected.SizeGiB < 1 || selected.SizeGiB > 1024 || len(kubevalidation.IsDNS1123Subdomain(selected.StorageClass)) != 0 {
			return nil, nil, ErrRuntimePreparation
		}
		if err = s.kubernetesAuthority(ctx, actor, true, true, row.Plan.MonthlyBudgetUSD); err != nil {
			return nil, nil, err
		}
		storage, err := target.Client.StorageV1().StorageClasses().Get(ctx, selected.StorageClass, metav1.GetOptions{})
		if err != nil || storage == nil || storage.UID == "" || storage.ResourceVersion == "" || storage.DeletionTimestamp != nil || storage.Provisioner == "" || storage.Provisioner == "kubernetes.io/no-provisioner" || storage.AllowedTopologies != nil && len(storage.AllowedTopologies) > 16 {
			return nil, nil, ErrRuntimePreparation
		}
		mode := "Immediate"
		if storage.VolumeBindingMode != nil {
			mode = string(*storage.VolumeBindingMode)
		}
		if mode != "Immediate" && mode != "WaitForFirstConsumer" || selected.Purpose != "" && mode != "Immediate" {
			return nil, nil, ErrRuntimePreparation
		}
		reclaim := "Delete"
		if storage.ReclaimPolicy != nil {
			reclaim = string(*storage.ReclaimPolicy)
		}
		if reclaim != "Retain" && reclaim != "Delete" {
			return nil, nil, ErrRuntimePreparation
		}
		volume := RuntimeVolumePlan{RuntimeVolumeSelection: selected, ClaimName: runtimeClaimName(row.Plan.ID, selected.ServiceID), StorageClassUID: string(storage.UID), StorageClassVersion: storage.ResourceVersion, BindingMode: mode, ReclaimPolicy: reclaim}
		var existing corev1.PersistentVolumeClaim
		if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "persistentvolumeclaims", volume.ClaimName, nil, &existing); !errors.Is(err, deploymentkube.ErrPreparationAbsent) {
			return nil, nil, errors.New("The new owned model claim name is occupied or unconfirmed; no volume was overwritten")
		}
		volumes[selected.ServiceID] = volume
		plan.Volumes = append(plan.Volumes, volume)
	}
	modelServices := map[string]bool{}
	var totalBytes int64
	for _, source := range plan.Models {
		volume := volumes[source.ServiceID]
		service := runtimeTemplateService(row, source.ServiceID)
		if validateRuntimeModelSource(source) != nil || volume.Purpose != "" || volume.ClaimName == "" || service == nil || service.Model == nil || !runtimeModelSafeName(service.Model.RelativePath) || modelServices[source.ServiceID] || source.Bytes > (volume.SizeGiB<<30)-(64<<20) {
			return nil, nil, ErrRuntimePreparation
		}
		modelServices[source.ServiceID] = true
		totalBytes += source.Bytes
		if totalBytes > 8<<30 {
			return nil, nil, ErrRuntimePreparation
		}
		plan.ModelsRelativePaths[source.ServiceID] = service.Model.RelativePath
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, nil, err
		}
		generation, err := kubeOpaqueID("secret-")
		if err != nil {
			return nil, nil, err
		}
		name := runtimeTransferSecret(row.Plan.ID, source.ServiceID)
		plan.Secrets = append(plan.Secrets, RuntimeSecretPlan{Name: name, Kind: "transfer-key", Keys: []string{"transfer.key"}, GenerationID: generation, ServiceID: source.ServiceID})
		private = append(private, runtimePrivateSecret{Name: name, Type: "Opaque", GenerationID: generation, Data: map[string][]byte{"transfer.key": []byte(hex.EncodeToString(key))}})
	}
	for serviceID, volume := range volumes {
		if volume.Purpose == "" && !modelServices[serviceID] {
			return nil, nil, errors.New("New model storage needs its explicitly reviewed initial file source; existing volumes are not overwritten")
		}
	}
	if len(plan.Models) > 0 {
		if !kubeImmutableImage.MatchString(input.ModelSinkBuilderImage) {
			return nil, nil, errors.New("Choose an existing immutable Go 1.25+ builder image for the fixed transfer helper")
		}
		plan.ModelSinkBuilderImage = input.ModelSinkBuilderImage
		plan.ModelSinkSourceSHA256 = runtimeModelSinkProfile()
	} else if input.ModelSinkBuilderImage != "" {
		return nil, nil, ErrRuntimePreparation
	}
	if input.GenerateTelemetryTLS {
		public, values, err := runtimeTLSSecrets(row.Template)
		if err != nil || len(public) == 0 {
			return nil, nil, ErrRuntimePreparation
		}
		plan.Secrets = append(plan.Secrets, public...)
		private = append(private, values...)
	}
	blob, blobPlan, blobPrivate, err := s.reviewRuntimeBlobSecret(ctx, actor, row, input.TelemetryBlobJobID)
	if err != nil {
		return nil, nil, err
	}
	if blob != nil {
		plan.TelemetryBlob = blob
		plan.Secrets = append(plan.Secrets, *blobPlan)
		private = append(private, *blobPrivate)
	}
	_, registryLedger, err := s.readRuntimeRegistries(actor.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	var credentials *privateRuntimeRegistry
	for i := range registryLedger.Registries {
		if registryLedger.Registries[i].RuntimeRegistry == row.Plan.Registry {
			credentials = &registryLedger.Registries[i]
		}
	}
	if credentials == nil {
		return nil, nil, ErrRuntimePreparation
	}
	if input.GenerateImagePullSecret {
		if credentials.Username == "" || credentials.Password == "" {
			return nil, nil, errors.New("Anonymous registry setup has no credential to copy into a pull Secret")
		}
		plan.ImagePullSecret = runtimeImagePullSecret(row.Plan.ID)
		plan.ImagePullServiceIDs = append([]string{}, input.ImagePullServiceIDs...)
		seen := map[string]bool{}
		for _, id := range plan.ImagePullServiceIDs {
			service := runtimeTemplateService(row, id)
			if service == nil || seen[id] {
				return nil, nil, ErrRuntimePreparation
			}
			seen[id] = true
			built := false
			for _, b := range row.Plan.Builds {
				if b.Selection.ServiceID == id {
					built = true
				}
			}
			if !built && !strings.HasPrefix(service.Image, row.Plan.Registry.Repository+"@sha256:") {
				return nil, nil, errors.New("Select only this repository's explicitly reviewed service images for its pull credential")
			}
			if len(service.ImagePullSecrets) >= 8 {
				return nil, nil, ErrRuntimePreparation
			}
		}
		for _, build := range row.Plan.Builds {
			if !seen[build.Selection.ServiceID] {
				return nil, nil, errors.New("Select every new private-registry image consumer for the reviewed pull Secret")
			}
		}
		name, _ := reference.ParseNormalizedNamed(row.Plan.Registry.Repository)
		host := reference.Domain(name)
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + credentials.Password))
		data, err := json.Marshal(map[string]any{"auths": map[string]any{host: map[string]string{"username": credentials.Username, "password": credentials.Password, "auth": encoded}}})
		if err != nil {
			return nil, nil, err
		}
		generation, err := kubeOpaqueID("secret-")
		if err != nil {
			return nil, nil, err
		}
		plan.Secrets = append(plan.Secrets, RuntimeSecretPlan{Name: plan.ImagePullSecret, Kind: "image-pull", Keys: []string{".dockerconfigjson"}, GenerationID: generation})
		private = append(private, runtimePrivateSecret{Name: plan.ImagePullSecret, Type: string(corev1.SecretTypeDockerConfigJson), GenerationID: generation, Data: map[string][]byte{".dockerconfigjson": data}})
	} else if len(input.ImagePullServiceIDs) > 0 || credentials.Username != "" && (len(row.Plan.Builds) > 0 || len(plan.Models) > 0) {
		return nil, nil, errors.New("Private image consumers require the explicit reviewed pull Secret; no ambient node login is assumed")
	}
	if len(plan.Volumes) == 0 && len(plan.Secrets) == 0 {
		return nil, nil, ErrRuntimePreparation
	}
	if len(plan.Secrets) > 32 {
		return nil, nil, ErrRuntimePreparation
	}
	names := map[string]bool{}
	for _, secret := range plan.Secrets {
		if names[secret.Name] {
			return nil, nil, errors.New("Generated Secret roles require distinct exact names")
		}
		names[secret.Name] = true
		var existing corev1.Secret
		if err = deploymentkube.PreparationResourceRPC(ctx, target, "GET", "secrets", secret.Name, nil, &existing); !errors.Is(err, deploymentkube.ErrPreparationAbsent) {
			return nil, nil, errors.New("A generated Secret name is occupied or unconfirmed; existing credentials are never replaced")
		}
	}
	if err = s.kubernetesAuthority(ctx, actor, true, true, row.Plan.MonthlyBudgetUSD); err != nil {
		return nil, nil, err
	}
	// No resource creation occurred. Values remain only in the private original
	// preparation; public plan/digest contains metadata plus random generation IDs.
	return plan, private, nil
}
