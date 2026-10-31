package product

import (
	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

type RuntimeVolumeSelection struct {
	ServiceID    string `json:"serviceId"`
	StorageClass string `json:"storageClass"`
	SizeGiB      int64  `json:"sizeGiB"`
	Purpose      string `json:"purpose,omitempty"`
}
type RuntimeResourceSelection struct {
	TelemetryBlobJobID      string                   `json:"telemetryBlobJobId,omitempty"`
	Volumes                 []RuntimeVolumeSelection `json:"volumes,omitempty"`
	Models                  []RuntimeModelSource     `json:"models,omitempty"`
	GenerateTelemetryTLS    bool                     `json:"generateTelemetryTls"`
	GenerateImagePullSecret bool                     `json:"generateImagePullSecret"`
	ImagePullServiceIDs     []string                 `json:"imagePullServiceIds,omitempty"`
	ModelSinkBuilderImage   string                   `json:"modelSinkBuilderImage,omitempty"`
	AcknowledgeNewResources bool                     `json:"acknowledgeNewResources"`
}
type RuntimeVolumePlan struct {
	RuntimeVolumeSelection
	ClaimName           string `json:"claimName"`
	StorageClassUID     string `json:"storageClassUid"`
	StorageClassVersion string `json:"storageClassVersion"`
	BindingMode         string `json:"bindingMode"`
	ReclaimPolicy       string `json:"reclaimPolicy"`
}
type RuntimeSecretPlan struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"` // telemetry-server, telemetry-client, image-pull, transfer-key
	Keys         []string `json:"keys"`
	GenerationID string   `json:"generationId"`
	ExpiresAt    string   `json:"expiresAt,omitempty"`
	ServiceID    string   `json:"serviceId,omitempty"`
}
type RuntimeResourcePlan struct {
	TelemetryBlob         *RuntimeTelemetryBlobBinding `json:"telemetryBlob,omitempty"`
	TargetID              string                       `json:"targetId"`
	TargetFingerprint     string                       `json:"targetFingerprint"`
	Namespace             string                       `json:"namespace"`
	NamespaceUID          string                       `json:"namespaceUid"`
	Volumes               []RuntimeVolumePlan          `json:"volumes"`
	Models                []RuntimeModelSource         `json:"models"`
	Secrets               []RuntimeSecretPlan          `json:"secrets"`
	ImagePullSecret       string                       `json:"imagePullSecret,omitempty"`
	ImagePullServiceIDs   []string                     `json:"imagePullServiceIds,omitempty"`
	ModelSinkBuilderImage string                       `json:"modelSinkBuilderImage,omitempty"`
	ModelSinkSourceSHA256 string                       `json:"modelSinkSourceSha256,omitempty"`
	ModelsRelativePaths   map[string]string            `json:"modelsRelativePaths,omitempty"`
}

// Private byte values never enter RuntimePreparationPlan/task/UI/evidence.
type runtimePrivateSecret struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Data         map[string][]byte `json:"data"`
	GenerationID string            `json:"generationId"`
}
type RuntimeOwnedResource struct {
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	DesiredDigest   string `json:"desiredDigest"` // metadata only, never secret values
	Attempted       bool   `json:"attempted"`
	UID             string `json:"uid,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	VolumeName      string `json:"volumeName,omitempty"`
	State           string `json:"state"`
	DeleteAttempted bool   `json:"deleteAttempted,omitempty"`
	AbsentObserved  bool   `json:"absentObserved,omitempty"`
}
type RuntimeModelTransfer struct {
	ServiceID        string                           `json:"serviceId"`
	OperationID      string                           `json:"operationId"`
	PodName          string                           `json:"podName"`
	PodUID           string                           `json:"podUid,omitempty"`
	PVCUID           string                           `json:"pvcUid,omitempty"`
	ContainerID      string                           `json:"containerId,omitempty"`
	ContainerImageID string                           `json:"containerImageId,omitempty"`
	UploadAttempted  bool                             `json:"uploadAttempted"`
	Download         *RuntimeModelDownloadProof       `json:"download,omitempty"`
	Receipt          *deploymentkube.ModelSinkReceipt `json:"receipt,omitempty"`
	OriginalZeroExit bool                             `json:"originalZeroExit"`
	State            string                           `json:"state"`
}
type RuntimeResourceOperation struct {
	Source               RuntimeBuildAttempt     `json:"source"`
	ApplyAttempted       bool                    `json:"applyAttempted"`
	Revision             int                     `json:"revision"`
	State                string                  `json:"state"`
	HelperBuildAttempted bool                    `json:"helperBuildAttempted"`
	HelperBuild          *previewruntime.Outcome `json:"helperBuild,omitempty"`
	Resources            []RuntimeOwnedResource  `json:"resources"`
	Models               []RuntimeModelTransfer  `json:"models"`
	ObservedAt           string                  `json:"observedAt"`
}
