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

func checkFenceBinding(req backend.CheckFenceRequest) string {
	req.ExpectedOriginalRef = ""
	data, _ := json.Marshal(req)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func checkFenceConfiguration(req backend.CheckFenceRequest) (*container.Config, *container.HostConfig) {
	// The inert execution shape is exactly the existing never-executable
	// directory marker. Only its typed ownership labels and logical name differ.
	_, host := fenceConfiguration(backend.LaunchFenceRequest{})
	labels := checkIdentityLabels(req.Identity)
	labels[backend.LabelLaunchFence] = "1"
	labels[fenceIntentLabel], labels[fenceBindingLabel] = req.IntentID, checkFenceBinding(req)
	return &container.Config{Cmd: []string{"/"}, NetworkDisabled: true, Labels: labels}, host
}

func checkFenceConfigurationDigest(req backend.CheckFenceRequest) string {
	config, host := checkFenceConfiguration(req)
	data, _ := json.Marshal(struct {
		Protocol int
		Config   *container.Config
		Host     *container.HostConfig
	}{backend.CheckExecutionProtocol, config, host})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// This pure recognizer extends only the shared inert-marker shape check. It
// grants neither ownership nor permission to retire a check. Agent labels and
// proofs remain unchanged; check markers never pretend to be agent sessions.
func inertCheckFenceLabels(row container.InspectResponse) bool {
	if row.Config == nil || row.ContainerJSONBase == nil {
		return false
	}
	labels := row.Config.Labels
	protocol, err1 := strconv.Atoi(labels[checkProtocolLabel])
	index, err2 := strconv.Atoi(labels[checkIndexLabel])
	identity := backend.CheckIdentity{Protocol: protocol, EngineID: labels[checkEngineLabel],
		InstanceID: labels[backend.LabelInstance], AllocationID: labels[checkAllocationLabel], CheckID: labels[checkIDLabel],
		CommandIndex: index, ScopeDigest: labels[checkScopeLabel], ProjectID: labels[checkProjectLabel], CommandDigest: labels[checkCommandLabel]}
	if err1 != nil || err2 != nil || !validCheckIdentity(identity, true) ||
		!fenceHex(labels[fenceIntentLabel], 16) || !fenceHex(labels[fenceBindingLabel], 32) || row.Name != "/"+checkName(identity) {
		return false
	}
	expected := checkIdentityLabels(identity)
	expected[backend.LabelLaunchFence] = "1"
	expected[fenceIntentLabel], expected[fenceBindingLabel] = labels[fenceIntentLabel], labels[fenceBindingLabel]
	return reflect.DeepEqual(expected, labels)
}

func validCheckFenceContainer(req backend.CheckFenceRequest, row container.InspectResponse) bool {
	expected, _ := checkFenceConfiguration(req)
	return inertFenceContainer(row) && reflect.DeepEqual(row.Config.Labels, expected.Labels)
}

func (b *Backend) checkFenceRequest(ctx context.Context, req backend.CheckFenceRequest) error {
	if !fenceHex(req.IntentID, 16) || req.ExpectedOriginalRef != "" && !fenceHex(req.ExpectedOriginalRef, 32) {
		return backend.ErrLaunchFenceOwnership
	}
	return b.checkIdentity(ctx, req.Identity)
}

func (b *Backend) checkAttemptInventory(ctx context.Context, req backend.CheckFenceRequest, allowedRef string) error {
	rows, err := b.api.ContainerList(ctx, container.ListOptions{All: true, Limit: 2, Filters: filters.NewArgs(
		filters.Arg("label", backend.LabelInstance+"="+req.Identity.InstanceID),
		filters.Arg("label", checkAllocationLabel+"="+req.Identity.AllocationID))})
	if err != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	if allowedRef == "" && len(rows) != 0 || allowedRef != "" && (len(rows) != 1 || rows[0].ID != allowedRef) {
		return backend.ErrLaunchFenceOwnership
	}
	for _, row := range rows {
		for key, value := range checkIdentityLabels(req.Identity) {
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

func (b *Backend) FenceCheckLaunch(ctx context.Context, req backend.CheckFenceRequest) (backend.CheckFenceProof, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := b.checkFenceRequest(ctx, req); err != nil {
		return backend.CheckFenceProof{}, err
	}
	name := checkName(req.Identity)
	original, err := b.api.ContainerInspect(ctx, name)
	missing := err != nil && ctx.Err() == nil && stoppedCleanupNotFound(err)
	if err != nil && !missing {
		return backend.CheckFenceProof{}, backend.ErrUnavailable
	}
	replacing, allowedRef := "", ""
	if !missing {
		if original.ContainerJSONBase == nil {
			return backend.CheckFenceProof{}, backend.ErrLaunchFenceOwnership
		}
		allowedRef = original.ID
		if fenceMarked(original) {
			if !validCheckFenceContainer(req, original) {
				return backend.CheckFenceProof{}, backend.ErrLaunchFenceOwnership
			}
			replacing = original.ID
		} else if !checkOwnedReference(req.Identity, original.ID, original) || req.ExpectedOriginalRef != "" && original.ID != req.ExpectedOriginalRef {
			return backend.CheckFenceProof{}, backend.ErrLaunchFenceOwnership
		}
	}
	if err = b.fenceInventory(ctx, replacing); err != nil {
		return backend.CheckFenceProof{}, err
	}
	if err = b.checkAttemptInventory(ctx, req, allowedRef); err != nil {
		return backend.CheckFenceProof{}, err
	}
	if !missing {
		fresh, inspectErr := b.api.ContainerInspect(ctx, original.ID)
		if inspectErr != nil || fresh.ContainerJSONBase == nil || fresh.ID != original.ID ||
			(replacing != "" && !validCheckFenceContainer(req, fresh)) ||
			(replacing == "" && !checkOwnedReference(req.Identity, original.ID, fresh)) {
			return backend.CheckFenceProof{}, backend.ErrLaunchFenceOwnership
		}
		if err = b.checkFenceRequest(ctx, req); err != nil {
			return backend.CheckFenceProof{}, err
		}
		// Only this explicitly reviewed recovery may force removal of the
		// original. Unacknowledged inert markers are always removed non-force.
		if err = b.api.ContainerRemove(ctx, original.ID, container.RemoveOptions{Force: replacing == "", RemoveVolumes: true}); err != nil || ctx.Err() != nil {
			return backend.CheckFenceProof{}, backend.ErrLaunchFencePending
		}
	}
	if err = b.checkFenceRequest(ctx, req); err != nil {
		return backend.CheckFenceProof{}, err
	}
	config, host := checkFenceConfiguration(req)
	created, err := b.api.ContainerCreate(ctx, config, host, nil, nil, name)
	if err != nil || ctx.Err() != nil || !fenceHex(created.ID, 32) {
		// A real response is required. Inspection of an unacknowledged marker
		// cannot stand in for ACK because late daemon rollback may remove it.
		return backend.CheckFenceProof{}, backend.ErrLaunchFencePending
	}
	ack := time.Now().UTC().Format(time.RFC3339Nano)
	proof := backend.CheckFenceProof{Identity: req.Identity, IntentID: req.IntentID, FenceRef: created.ID,
		ConfigDigest: checkFenceConfigurationDigest(req), AcknowledgedAt: ack, ObservedAt: ack}
	if err = b.VerifyCheckLaunchFence(ctx, req, proof); err != nil {
		return backend.CheckFenceProof{}, err
	}
	proof.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return proof, nil
}

func (b *Backend) VerifyCheckLaunchFence(ctx context.Context, req backend.CheckFenceRequest, proof backend.CheckFenceProof) error {
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	if err := b.checkFenceRequest(ctx, req); err != nil {
		return err
	}
	ack, err1 := time.Parse(time.RFC3339Nano, proof.AcknowledgedAt)
	observed, err2 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
	if proof.Identity != req.Identity || proof.IntentID != req.IntentID || !fenceHex(proof.FenceRef, 32) ||
		proof.ConfigDigest != checkFenceConfigurationDigest(req) || err1 != nil || err2 != nil ||
		ack.IsZero() || observed.Before(ack) || observed.After(time.Now().UTC()) {
		return backend.ErrLaunchFenceOwnership
	}
	row, err := b.api.ContainerInspect(ctx, proof.FenceRef)
	if err != nil || row.ContainerJSONBase == nil || row.ID != proof.FenceRef || !validCheckFenceContainer(req, row) {
		return backend.ErrLaunchFenceOwnership
	}
	byName, err := b.api.ContainerInspect(ctx, checkName(req.Identity))
	if err != nil || byName.ContainerJSONBase == nil || byName.ID != proof.FenceRef || !validCheckFenceContainer(req, byName) {
		return backend.ErrLaunchFenceOwnership
	}
	if err = b.checkAttemptInventory(ctx, req, proof.FenceRef); err != nil {
		return err
	}
	return b.checkFenceRequest(ctx, req)
}
