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

const documentAllocationLabel = "luci.document-allocation"

func documentFenceBinding(req backend.DocumentExtractionFenceRequest) string {
	req.ExpectedOriginalRef = ""
	data, _ := json.Marshal(req)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func documentFenceConfiguration(req backend.DocumentExtractionFenceRequest) (*container.Config, *container.HostConfig) {
	_, host := fenceConfiguration(backend.LaunchFenceRequest{})
	labels := documentLabels(req.Identity)
	labels[backend.LabelLaunchFence] = "1"
	labels[fenceIntentLabel] = req.IntentID
	labels[fenceBindingLabel] = documentFenceBinding(req)
	return &container.Config{Cmd: []string{"/"}, NetworkDisabled: true, Labels: labels}, host
}
func documentFenceConfigurationDigest(req backend.DocumentExtractionFenceRequest) string {
	config, host := documentFenceConfiguration(req)
	data, _ := json.Marshal(struct {
		Protocol int
		Config   *container.Config
		Host     *container.HostConfig
	}{backend.DocumentExtractionProtocol, config, host})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// Pure label recognizer for the shared inert marker shape validator. It never
// authorizes a start, an allocation adoption, retirement or capacity release.
func inertDocumentFenceLabels(row container.InspectResponse) bool {
	if row.Config == nil || row.ContainerJSONBase == nil {
		return false
	}
	labels := row.Config.Labels
	protocol, err := strconv.Atoi(labels["luci.document-protocol"])
	identity := backend.DocumentExtractionIdentity{Protocol: protocol, Profile: labels["luci.document-profile"], EngineID: labels["luci.document-engine"], InstanceID: labels[backend.LabelInstance], AllocationID: labels[documentAllocationLabel], ScopeDigest: labels["luci.document-scope"], ActorDigest: labels["luci.document-actor"], ProjectID: labels["luci.document-project"], DocumentID: labels["luci.document-id"], AttemptID: labels["luci.document-attempt"], OriginalDigest: labels["luci.document-original"], CommandDigest: labels["luci.document-command"]}
	if err != nil || !backend.ValidateDocumentExtractionIdentity(identity, true) || !fenceHex(labels[fenceIntentLabel], 16) || !fenceHex(labels[fenceBindingLabel], 32) || row.Name != "/"+documentName(identity) {
		return false
	}
	expected := documentLabels(identity)
	expected[backend.LabelLaunchFence] = "1"
	expected[fenceIntentLabel] = labels[fenceIntentLabel]
	expected[fenceBindingLabel] = labels[fenceBindingLabel]
	return reflect.DeepEqual(expected, labels)
}
func validDocumentFenceContainer(req backend.DocumentExtractionFenceRequest, row container.InspectResponse) bool {
	expected, _ := documentFenceConfiguration(req)
	return inertFenceContainer(row) && reflect.DeepEqual(row.Config.Labels, expected.Labels)
}
func (b *DocumentExtractor) documentFenceRequest(ctx context.Context, req backend.DocumentExtractionFenceRequest) error {
	if !fenceHex(req.IntentID, 16) || req.ExpectedOriginalRef != "" && !fenceHex(req.ExpectedOriginalRef, 32) {
		return backend.ErrLaunchFenceOwnership
	}
	return b.documentIdentity(ctx, req.Identity)
}

func (b *DocumentExtractor) documentAttemptInventory(ctx context.Context, req backend.DocumentExtractionFenceRequest, allowedRef string) error {
	rows, err := b.api.ContainerList(ctx, container.ListOptions{All: true, Limit: 2, Filters: filters.NewArgs(
		filters.Arg("label", backend.LabelInstance+"="+req.Identity.InstanceID),
		filters.Arg("label", documentAllocationLabel+"="+req.Identity.AllocationID))})
	if err != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	if allowedRef == "" && len(rows) != 0 || allowedRef != "" && (len(rows) != 1 || rows[0].ID != allowedRef) {
		return backend.ErrLaunchFenceOwnership
	}
	for _, row := range rows {
		for key, value := range documentLabels(req.Identity) {
			if row.Labels[key] != value {
				return backend.ErrLaunchFenceOwnership
			}
		}
	}
	if req.ExpectedOriginalRef != "" && req.ExpectedOriginalRef != allowedRef {
		_, err := b.api.ContainerInspect(ctx, req.ExpectedOriginalRef)
		if err == nil || ctx.Err() != nil || !stoppedCleanupNotFound(err) {
			return backend.ErrLaunchFenceOwnership
		}
	}
	return nil
}

func (b *DocumentExtractor) FenceDocumentLaunch(ctx context.Context, req backend.DocumentExtractionFenceRequest) (backend.DocumentExtractionFenceProof, error) {
	ctx, cancel := context.WithTimeout(ctx, backend.DocumentCleanupBudget)
	defer cancel()
	if err := b.documentFenceRequest(ctx, req); err != nil {
		return backend.DocumentExtractionFenceProof{}, err
	}
	name := documentName(req.Identity)
	original, err := b.api.ContainerInspect(ctx, name)
	missing := err != nil && ctx.Err() == nil && stoppedCleanupNotFound(err)
	if err != nil && !missing {
		return backend.DocumentExtractionFenceProof{}, backend.ErrUnavailable
	}
	replacing, allowedRef := "", ""
	if !missing {
		if original.ContainerJSONBase == nil {
			return backend.DocumentExtractionFenceProof{}, backend.ErrLaunchFenceOwnership
		}
		allowedRef = original.ID
		if fenceMarked(original) {
			if !validDocumentFenceContainer(req, original) {
				return backend.DocumentExtractionFenceProof{}, backend.ErrLaunchFenceOwnership
			}
			replacing = original.ID
		} else if !documentOwnedReference(req.Identity, original.ID, original) || req.ExpectedOriginalRef != "" && original.ID != req.ExpectedOriginalRef {
			return backend.DocumentExtractionFenceProof{}, backend.ErrLaunchFenceOwnership
		}
	}
	if err = b.fenceInventory(ctx, replacing); err != nil {
		return backend.DocumentExtractionFenceProof{}, err
	}
	if err = b.documentAttemptInventory(ctx, req, allowedRef); err != nil {
		return backend.DocumentExtractionFenceProof{}, err
	}
	if !missing {
		fresh, inspectErr := b.api.ContainerInspect(ctx, original.ID)
		if inspectErr != nil || fresh.ContainerJSONBase == nil || fresh.ID != original.ID ||
			(replacing != "" && !validDocumentFenceContainer(req, fresh)) ||
			(replacing == "" && !documentOwnedReference(req.Identity, original.ID, fresh)) {
			return backend.DocumentExtractionFenceProof{}, backend.ErrLaunchFenceOwnership
		}
		if err = b.documentFenceRequest(ctx, req); err != nil {
			return backend.DocumentExtractionFenceProof{}, err
		}
		// Only this explicitly reviewed recovery may force removal of the
		// original. Unacknowledged inert markers are always removed non-force.
		if err = b.api.ContainerRemove(ctx, original.ID, container.RemoveOptions{Force: replacing == "", RemoveVolumes: true}); err != nil || ctx.Err() != nil {
			return backend.DocumentExtractionFenceProof{}, backend.ErrLaunchFencePending
		}
	}
	if err = b.documentFenceRequest(ctx, req); err != nil {
		return backend.DocumentExtractionFenceProof{}, err
	}
	config, host := documentFenceConfiguration(req)
	created, err := b.api.ContainerCreate(ctx, config, host, nil, nil, name)
	if err != nil || ctx.Err() != nil || !fenceHex(created.ID, 32) {
		// A real response is required. Inspection of an unacknowledged marker
		// cannot stand in for ACK because late daemon rollback may remove it.
		return backend.DocumentExtractionFenceProof{}, backend.ErrLaunchFencePending
	}
	ack := time.Now().UTC().Format(time.RFC3339Nano)
	proof := backend.DocumentExtractionFenceProof{Identity: req.Identity, IntentID: req.IntentID, FenceRef: created.ID,
		ConfigDigest: documentFenceConfigurationDigest(req), AcknowledgedAt: ack, ObservedAt: ack}
	if err = b.VerifyDocumentLaunchFence(ctx, req, proof); err != nil {
		return backend.DocumentExtractionFenceProof{}, err
	}
	proof.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return proof, nil
}

func (b *DocumentExtractor) VerifyDocumentLaunchFence(ctx context.Context, req backend.DocumentExtractionFenceRequest, proof backend.DocumentExtractionFenceProof) error {
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	if err := b.documentFenceRequest(ctx, req); err != nil {
		return err
	}
	ack, err1 := time.Parse(time.RFC3339Nano, proof.AcknowledgedAt)
	observed, err2 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
	if proof.Identity != req.Identity || proof.IntentID != req.IntentID || !fenceHex(proof.FenceRef, 32) ||
		proof.ConfigDigest != documentFenceConfigurationDigest(req) || err1 != nil || err2 != nil ||
		ack.IsZero() || observed.Before(ack) || observed.After(time.Now().UTC()) {
		return backend.ErrLaunchFenceOwnership
	}
	row, err := b.api.ContainerInspect(ctx, proof.FenceRef)
	if err != nil || row.ContainerJSONBase == nil || row.ID != proof.FenceRef || !validDocumentFenceContainer(req, row) {
		return backend.ErrLaunchFenceOwnership
	}
	byName, err := b.api.ContainerInspect(ctx, documentName(req.Identity))
	if err != nil || byName.ContainerJSONBase == nil || byName.ID != proof.FenceRef || !validDocumentFenceContainer(req, byName) {
		return backend.ErrLaunchFenceOwnership
	}
	if err = b.documentAttemptInventory(ctx, req, proof.FenceRef); err != nil {
		return err
	}
	return b.documentFenceRequest(ctx, req)
}
