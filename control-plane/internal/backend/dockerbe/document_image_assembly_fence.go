package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const assemblyAllocationLabel = "luci.assembly-allocation"

func assemblyFenceBinding(req backend.DocumentImageAssemblyFenceRequest) string {
	req.ExpectedRef = ""
	data, _ := json.Marshal(req)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func assemblyFenceConfiguration(req backend.DocumentImageAssemblyFenceRequest) (*container.Config, *container.HostConfig) {
	_, host := fenceConfiguration(backend.LaunchFenceRequest{})
	labels := assemblyLabels(req.Identity)
	labels[backend.LabelLaunchFence] = "1"
	labels[fenceIntentLabel] = req.IntentID
	labels[fenceBindingLabel] = assemblyFenceBinding(req)
	return &container.Config{Cmd: []string{"/"}, NetworkDisabled: true, Labels: labels}, host
}
func assemblyFenceConfigurationDigest(req backend.DocumentImageAssemblyFenceRequest) string {
	config, host := assemblyFenceConfiguration(req)
	data, _ := json.Marshal(struct {
		Protocol int
		Config   *container.Config
		Host     *container.HostConfig
	}{backend.DocumentImageAssemblyProtocol, config, host})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// Pure label recognizer for the shared inert marker shape validator. It never
// authorizes a start, an allocation adoption, retirement or capacity release.
func inertDocumentImageAssemblyFenceLabels(row container.InspectResponse) bool {
	if row.Config == nil || row.ContainerJSONBase == nil {
		return false
	}
	labels := row.Config.Labels
	protocol, err := strconv.Atoi(labels["luci.assembly-protocol"])
	identity := backend.DocumentImageAssemblyIdentity{Protocol: protocol, Profile: labels["luci.assembly-profile"], EngineID: labels["luci.assembly-engine"], InstanceID: labels[backend.LabelInstance], AllocationID: labels[assemblyAllocationLabel], ScopeDigest: labels["luci.assembly-scope"], ActorDigest: labels["luci.assembly-actor"], ProjectID: labels["luci.assembly-project"], SourceCommit: labels["luci.assembly-source-commit"], SourceDigest: labels["luci.assembly-source"], InputDigest: labels["luci.assembly-input"], HelperDigest: labels["luci.assembly-helper"], RecipeDigest: labels["luci.assembly-recipe"], CommandDigest: labels["luci.assembly-command"]}
	if err != nil || !backend.ValidateDocumentImageAssemblyIdentity(identity, true) || !fenceHex(labels[fenceIntentLabel], 16) || !fenceHex(labels[fenceBindingLabel], 32) || row.Name != "/"+assemblyName(identity) {
		return false
	}
	expected := assemblyLabels(identity)
	expected[backend.LabelLaunchFence] = "1"
	expected[fenceIntentLabel] = labels[fenceIntentLabel]
	expected[fenceBindingLabel] = labels[fenceBindingLabel]
	return reflect.DeepEqual(expected, labels)
}
func validAssemblyFenceContainer(req backend.DocumentImageAssemblyFenceRequest, row container.InspectResponse) bool {
	expected, _ := assemblyFenceConfiguration(req)
	return inertFenceContainer(row) && reflect.DeepEqual(row.Config.Labels, expected.Labels)
}
func (b *DocumentImageAssembler) assemblyFenceRequest(ctx context.Context, req backend.DocumentImageAssemblyFenceRequest) error {
	if !fenceHex(req.IntentID, 16) || req.ExpectedRef != "" && !fenceHex(req.ExpectedRef, 32) {
		return backend.ErrLaunchFenceOwnership
	}
	return b.assemblyIdentity(ctx, req.Identity)
}

func (b *DocumentImageAssembler) assemblyAttemptInventory(ctx context.Context, req backend.DocumentImageAssemblyFenceRequest, allowedRef string) error {
	rows, err := b.api.ContainerList(ctx, container.ListOptions{All: true, Limit: 2, Filters: filters.NewArgs(
		filters.Arg("label", backend.LabelInstance+"="+req.Identity.InstanceID),
		filters.Arg("label", assemblyAllocationLabel+"="+req.Identity.AllocationID))})
	if err != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	if allowedRef == "" && len(rows) != 0 || allowedRef != "" && (len(rows) != 1 || rows[0].ID != allowedRef) {
		return backend.ErrLaunchFenceOwnership
	}
	for _, row := range rows {
		for key, value := range assemblyLabels(req.Identity) {
			if row.Labels[key] != value {
				return backend.ErrLaunchFenceOwnership
			}
		}
	}
	if req.ExpectedRef != "" && req.ExpectedRef != allowedRef {
		_, err := b.api.ContainerInspect(ctx, req.ExpectedRef)
		if err == nil || ctx.Err() != nil || !stoppedCleanupNotFound(err) {
			return backend.ErrLaunchFenceOwnership
		}
	}
	return nil
}

func (b *DocumentImageAssembler) FenceDocumentImageAssembly(ctx context.Context, req backend.DocumentImageAssemblyFenceRequest) (backend.DocumentImageAssemblyFenceProof, error) {
	ctx, cancel := context.WithTimeout(ctx, backend.DocumentImageAssemblyCleanupBudget)
	defer cancel()
	if err := b.assemblyFenceRequest(ctx, req); err != nil {
		return backend.DocumentImageAssemblyFenceProof{}, err
	}
	name := assemblyName(req.Identity)
	original, err := b.api.ContainerInspect(ctx, name)
	missing := err != nil && ctx.Err() == nil && stoppedCleanupNotFound(err)
	if err != nil && !missing {
		return backend.DocumentImageAssemblyFenceProof{}, backend.ErrUnavailable
	}
	replacing, allowedRef := "", ""
	if !missing {
		if original.ContainerJSONBase == nil {
			return backend.DocumentImageAssemblyFenceProof{}, backend.ErrLaunchFenceOwnership
		}
		allowedRef = original.ID
		if fenceMarked(original) {
			if !validAssemblyFenceContainer(req, original) {
				return backend.DocumentImageAssemblyFenceProof{}, backend.ErrLaunchFenceOwnership
			}
			replacing = original.ID
		} else if !assemblyOwnedReference(req.Identity, original.ID, original) || req.ExpectedRef != "" && original.ID != req.ExpectedRef {
			return backend.DocumentImageAssemblyFenceProof{}, backend.ErrLaunchFenceOwnership
		}
	}
	if err = b.fenceInventory(ctx, replacing); err != nil {
		return backend.DocumentImageAssemblyFenceProof{}, err
	}
	if err = b.assemblyAttemptInventory(ctx, req, allowedRef); err != nil {
		return backend.DocumentImageAssemblyFenceProof{}, err
	}
	if !missing {
		fresh, inspectErr := b.api.ContainerInspect(ctx, original.ID)
		if inspectErr != nil || fresh.ContainerJSONBase == nil || fresh.ID != original.ID ||
			(replacing != "" && !validAssemblyFenceContainer(req, fresh)) ||
			(replacing == "" && !assemblyOwnedReference(req.Identity, original.ID, fresh)) {
			return backend.DocumentImageAssemblyFenceProof{}, backend.ErrLaunchFenceOwnership
		}
		if err = b.assemblyFenceRequest(ctx, req); err != nil {
			return backend.DocumentImageAssemblyFenceProof{}, err
		}
		// Only this explicitly reviewed recovery may force removal of the
		// original. Unacknowledged inert markers are always removed non-force.
		if err = b.api.ContainerRemove(ctx, original.ID, container.RemoveOptions{Force: replacing == "", RemoveVolumes: true}); err != nil || ctx.Err() != nil {
			return backend.DocumentImageAssemblyFenceProof{}, backend.ErrLaunchFencePending
		}
	}
	if err = b.assemblyFenceRequest(ctx, req); err != nil {
		return backend.DocumentImageAssemblyFenceProof{}, err
	}
	config, host := assemblyFenceConfiguration(req)
	created, err := b.api.ContainerCreate(ctx, config, host, nil, nil, name)
	if err != nil || ctx.Err() != nil || !fenceHex(created.ID, 32) {
		// A real response is required. Inspection of an unacknowledged marker
		// cannot stand in for ACK because late daemon rollback may remove it.
		return backend.DocumentImageAssemblyFenceProof{}, backend.ErrLaunchFencePending
	}
	ack := time.Now().UTC().Format(time.RFC3339Nano)
	proof := backend.DocumentImageAssemblyFenceProof{Identity: req.Identity, IntentID: req.IntentID, FenceRef: created.ID,
		ConfigDigest: assemblyFenceConfigurationDigest(req), AcknowledgedAt: ack, ObservedAt: ack}
	if err = b.VerifyDocumentImageAssemblyFence(ctx, req, proof); err != nil {
		return backend.DocumentImageAssemblyFenceProof{}, err
	}
	proof.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if b.releaseAssemblySources(req.Identity) != nil {
		return backend.DocumentImageAssemblyFenceProof{}, backend.ErrUnavailable
	}
	return proof, nil
}

func (b *DocumentImageAssembler) VerifyDocumentImageAssemblyFence(ctx context.Context, req backend.DocumentImageAssemblyFenceRequest, proof backend.DocumentImageAssemblyFenceProof) error {
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	if err := b.assemblyFenceRequest(ctx, req); err != nil {
		return err
	}
	ack, err1 := time.Parse(time.RFC3339Nano, proof.AcknowledgedAt)
	observed, err2 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
	if proof.Identity != req.Identity || proof.IntentID != req.IntentID || !fenceHex(proof.FenceRef, 32) ||
		proof.ConfigDigest != assemblyFenceConfigurationDigest(req) || err1 != nil || err2 != nil ||
		ack.IsZero() || observed.Before(ack) || observed.After(time.Now().UTC()) {
		return backend.ErrLaunchFenceOwnership
	}
	row, err := b.api.ContainerInspect(ctx, proof.FenceRef)
	if err != nil || row.ContainerJSONBase == nil || row.ID != proof.FenceRef || !validAssemblyFenceContainer(req, row) {
		return backend.ErrLaunchFenceOwnership
	}
	byName, err := b.api.ContainerInspect(ctx, assemblyName(req.Identity))
	if err != nil || byName.ContainerJSONBase == nil || byName.ID != proof.FenceRef || !validAssemblyFenceContainer(req, byName) {
		return backend.ErrLaunchFenceOwnership
	}
	if err = b.assemblyAttemptInventory(ctx, req, proof.FenceRef); err != nil {
		return err
	}
	if err = b.assemblyFenceRequest(ctx, req); err != nil {
		return err
	}
	return b.releaseAssemblySources(req.Identity)
}
