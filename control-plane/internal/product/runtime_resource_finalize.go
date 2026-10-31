package product

import (
	"context"
	"encoding/json"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
)

// Closed optional telemetry schema projection keeps this resource engine
// independent of collector/query implementation. No arbitrary Pod or raw
// manifest is accepted. The owning telemetry compiler validates full fields.
func runtimeVolumeSelectionValid(service deploymentplan.Service, selected RuntimeVolumeSelection) bool {
	if service.Replicas != 1 || service.DeploymentStrategy != "recreate" {
		return false
	}
	if selected.Purpose == "" {
		return service.Kind == "inference" && service.Model != nil
	}
	if selected.Purpose != "telemetry-tsdb" && selected.Purpose != "telemetry-wal" {
		return false
	}
	var projection struct {
		TelemetryStorage *struct {
			Profile     string `json:"profile"`
			CapacityGiB int64  `json:"capacityGiB"`
		} `json:"telemetryStorage"`
	}
	raw, err := json.Marshal(service)
	if err != nil || json.Unmarshal(raw, &projection) != nil || projection.TelemetryStorage == nil || projection.TelemetryStorage.CapacityGiB != selected.SizeGiB {
		return false
	}
	profile := projection.TelemetryStorage.Profile
	return selected.Purpose == "telemetry-tsdb" && service.Kind == "metrics-store" && profile == "prometheus-3.15.1" || selected.Purpose == "telemetry-wal" && service.Kind == "trace-store" && profile == "tempo-2.10.10-azure"
}

func deriveRuntimeResourceReferences(row storedRuntimePreparation, next *deploymentplan.Blueprint) error {
	if row.Plan.Resources == nil {
		return nil
	}
	if row.Resources == nil || row.Resources.State != "ready" {
		return ErrRuntimePreparation
	}
	for _, volume := range row.Plan.Resources.Volumes {
		original := runtimeOwnedResource(row.Resources, "PersistentVolumeClaim", volume.ClaimName)
		if original == nil || original.UID == "" || original.VolumeName == "" || original.State != "bound" {
			return ErrRuntimePreparation
		}
		found := false
		for i := range next.Services {
			service := &next.Services[i]
			if service.ID != volume.ServiceID {
				continue
			}
			if !runtimeVolumeSelectionValid(*service, volume.RuntimeVolumeSelection) {
				return ErrRuntimePreparation
			}
			if volume.Purpose == "" {
				if service.Model == nil {
					return ErrRuntimePreparation
				}
				service.Model.ClaimName = volume.ClaimName
				service.Model.AccessGroup = 1000
			} else {
				// Only one explicitly selected claimName changes. All profile, retention,
				// TLS, destination and provider references stay exactly captured.
				raw, err := json.Marshal(service)
				if err != nil {
					return err
				}
				var object map[string]json.RawMessage
				if json.Unmarshal(raw, &object) != nil {
					return ErrRuntimePreparation
				}
				var storage map[string]json.RawMessage
				if json.Unmarshal(object["telemetryStorage"], &storage) != nil {
					return ErrRuntimePreparation
				}
				storage["claimName"], _ = json.Marshal(volume.ClaimName)
				object["telemetryStorage"], err = json.Marshal(storage)
				if err != nil {
					return err
				}
				raw, err = json.Marshal(object)
				if err != nil || json.Unmarshal(raw, service) != nil {
					return ErrRuntimePreparation
				}
			}
			found = true
		}
		if !found {
			return ErrRuntimePreparation
		}
	}
	for _, plan := range row.Plan.Resources.Secrets {
		owned := runtimeOwnedResource(row.Resources, "Secret", plan.Name)
		if owned == nil || owned.UID == "" || !owned.Attempted {
			return ErrRuntimePreparation
		}
	}
	for _, model := range row.Plan.Resources.Models {
		state := runtimeModelTransfer(row.Resources, model.ServiceID)
		if state == nil || !state.OriginalZeroExit || state.Download == nil || state.Receipt == nil {
			return ErrRuntimePreparation
		}
		pod := runtimeOwnedResource(row.Resources, "Pod", state.PodName)
		if pod == nil || !pod.DeleteAttempted || !pod.AbsentObserved {
			return ErrRuntimePreparation
		}
	}
	if name := row.Plan.Resources.ImagePullSecret; name != "" {
		for _, id := range row.Plan.Resources.ImagePullServiceIDs {
			found := false
			for i := range next.Services {
				if next.Services[i].ID == id {
					exists := false
					for _, old := range next.Services[i].ImagePullSecrets {
						if old == name {
							exists = true
						}
					}
					if !exists {
						next.Services[i].ImagePullSecrets = append(next.Services[i].ImagePullSecrets, name)
					}
					found = true
				}
			}
			if !found {
				return ErrRuntimePreparation
			}
		}
	}
	return nil
}

func (s *Server) observeRuntimeResourcesForFinalization(ctx context.Context, row storedRuntimePreparation) error {
	if row.Plan.Resources == nil {
		return nil
	}
	if row.Resources == nil || row.Resources.State != "ready" {
		return ErrRuntimePreparation
	}
	copy, err := cloneRuntimeResourceOperation(row.Resources)
	if err != nil {
		return err
	}
	row.Resources = copy
	// Inspection may normalize an in-memory observation but cannot change the
	// finalization CAS baseline or issue a create/build/transfer/cleanup.
	w := &runtimeResourceWriter{server: s, row: row, readOnly: true}
	if err = s.observeRuntimeResourceObjects(ctx, w); err != nil {
		return err
	}
	if len(row.Plan.Resources.Models) > 0 {
		if row.Resources.HelperBuild == nil || s.PreviewRuntime().Observe == nil {
			return ErrRuntimePreparation
		}
		got, err := s.PreviewRuntime().Observe(ctx, row.Resources.HelperBuild.Identity)
		if err != nil || got.ImmutableImage != row.Resources.HelperBuild.ImmutableImage || got.Phase != "published" || got.ResourcesHeld || !got.CleanupConfirmed {
			return ErrRuntimePreparation
		}
	}
	return s.runtimeResourceMetadataAuthority(ctx, row, false)
}
