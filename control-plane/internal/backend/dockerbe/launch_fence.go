package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const (
	fenceIntentLabel  = "luci.fence-intent"
	fenceBindingLabel = "luci.fence-binding"
	fenceMemory       = 6 << 20
	fenceCPU          = 100_000_000
)

var _ backend.LaunchFencer = (*Backend)(nil)

type fenceIdentityClient interface {
	Info(context.Context) (system.Info, error)
	DaemonHost() string
}

func localFenceEndpoint(value string) bool {
	if localDesktopEndpoint(value) {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "unix" && u.Host == "" && u.User == nil &&
		u.RawQuery == "" && u.Fragment == "" && u.RawPath == "" &&
		strings.HasPrefix(u.Path, "/") && path.Clean(u.Path) == u.Path &&
		u.Path != "/" && !strings.ContainsRune(u.Path, 0)
}

func (b *Backend) LaunchFenceIdentity(ctx context.Context) (backend.LaunchFenceIdentity, error) {
	api, ok := b.api.(fenceIdentityClient)
	if !ok || !localFenceEndpoint(api.DaemonHost()) {
		return backend.LaunchFenceIdentity{}, fmt.Errorf("launch recovery requires a local Docker socket or named pipe: %w", backend.ErrUnsupported)
	}
	info, err := api.Info(ctx)
	if err != nil || ctx.Err() != nil {
		return backend.LaunchFenceIdentity{}, fmt.Errorf("Docker launch identity is unavailable: %w", backend.ErrUnavailable)
	}
	if info.OSType != "linux" || info.ID == "" || len(info.ID) > 256 || strings.TrimSpace(info.ID) != info.ID {
		return backend.LaunchFenceIdentity{}, fmt.Errorf("Docker launch identity is unsupported: %w", backend.ErrUnsupported)
	}
	return backend.LaunchFenceIdentity{Protocol: backend.LaunchFenceProtocol, EngineID: info.ID, InstanceID: b.instance}, nil
}

func fenceHex(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size && strings.ToLower(value) == value
}

func fenceLogicalID(value string) bool {
	if value == "" || len(value) > 200 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func (b *Backend) fenceRequestIdentity(ctx context.Context, req backend.LaunchFenceRequest) (backend.LaunchFenceIdentity, error) {
	if !fenceLogicalID(req.SandboxID) || !fenceLogicalID(req.SessionID) || !fenceHex(req.IntentID, 16) ||
		!fenceHex(req.ScopeDigest, 32) || !fenceHex(req.OperationDigest, 32) ||
		(req.ExpectedOriginalRef != "" && !fenceHex(req.ExpectedOriginalRef, 32)) || req.InstanceID != b.instance {
		return backend.LaunchFenceIdentity{}, backend.ErrLaunchFenceOwnership
	}
	identity, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		return identity, err
	}
	if identity.EngineID != req.EngineID || identity.InstanceID != req.InstanceID {
		return identity, backend.ErrLaunchFenceOwnership
	}
	return identity, nil
}

func fenceRequestDigest(req backend.LaunchFenceRequest) string {
	// A newly learned original reference may strengthen the removal
	// precondition on retry. It does not change the immutable retirement
	// identity or permit relaxing an already saved nonempty reference.
	req.ExpectedOriginalRef = ""
	data, _ := json.Marshal(req)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func fenceConfiguration(req backend.LaunchFenceRequest) (*container.Config, *container.HostConfig) {
	limit := int64(1)
	return &container.Config{
			Cmd: []string{"/"}, NetworkDisabled: true,
			Labels: map[string]string{
				backend.LabelLaunchFence: "1", backend.LabelInstance: req.InstanceID,
				backend.LabelSandboxID: req.SandboxID, backend.LabelSessionID: req.SessionID,
				fenceIntentLabel: req.IntentID, fenceBindingLabel: fenceRequestDigest(req),
			},
		}, &container.HostConfig{
			ReadonlyRootfs: true, NetworkMode: "none", IpcMode: "none",
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
			CapDrop:       []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"},
			LogConfig: container.LogConfig{Type: "none"},
			Resources: container.Resources{Memory: fenceMemory, NanoCPUs: fenceCPU, PidsLimit: &limit},
		}
}

func fenceConfigurationDigest(req backend.LaunchFenceRequest) string {
	cfg, host := fenceConfiguration(req)
	data, _ := json.Marshal(struct {
		Protocol int
		Config   *container.Config
		Host     *container.HostConfig
	}{backend.LaunchFenceProtocol, cfg, host})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func fenceMarked(resp container.InspectResponse) bool {
	if resp.Config == nil {
		return false
	}
	_, marked := resp.Config.Labels[backend.LabelLaunchFence]
	return marked
}

func validFenceContainer(req backend.LaunchFenceRequest, resp container.InspectResponse) bool {
	expected, _ := fenceConfiguration(req)
	return inertFenceContainer(resp) && reflect.DeepEqual(resp.Config.Labels, expected.Labels)
}

// A created marker cannot execute its directory command and has no worker
// filesystem or mounts. This strict shape check permits resource observation
// to distinguish inert markers from active allocations without believing a
// label alone. It does not prove retirement or authorize removal.
func inertFenceContainer(resp container.InspectResponse) bool {
	if resp.ContainerJSONBase == nil || !fenceHex(resp.ID, 32) ||
		resp.State == nil || resp.State.Status != "created" || resp.State.Running || resp.State.Restarting ||
		resp.State.Paused || resp.State.Dead || resp.State.Pid != 0 || resp.Config == nil || resp.HostConfig == nil ||
		len(resp.Mounts) != 0 || resp.Image != "" || resp.Path != "/" || len(resp.Args) != 0 {
		return false
	}
	cfg, host := resp.Config, resp.HostConfig
	labels := cfg.Labels
	agentLabels := len(labels) == 6 && labels[backend.LabelLaunchFence] == "1" && backend.ValidateInstance(labels[backend.LabelInstance]) == nil &&
		fenceLogicalID(labels[backend.LabelSandboxID]) && fenceLogicalID(labels[backend.LabelSessionID]) &&
		fenceHex(labels[fenceIntentLabel], 16) && fenceHex(labels[fenceBindingLabel], 32) &&
		resp.Name == "/"+namePrefix+labels[backend.LabelSandboxID]
	if (!agentLabels && !inertCheckFenceLabels(resp) && !inertDocumentFenceLabels(resp)) ||
		cfg.Image != "" || len(cfg.Entrypoint) != 0 ||
		len(cfg.Cmd) != 1 || cfg.Cmd[0] != "/" || len(cfg.Env) != 0 || len(cfg.ExposedPorts) != 0 ||
		len(cfg.Volumes) != 0 || cfg.WorkingDir != "" || cfg.User != "" || cfg.Healthcheck != nil ||
		cfg.Tty || cfg.OpenStdin || !cfg.NetworkDisabled {
		return false
	}
	return host.ReadonlyRootfs && host.NetworkMode == "none" && host.IpcMode == "none" && !host.Privileged &&
		!host.AutoRemove && host.RestartPolicy.Name == container.RestartPolicyDisabled &&
		host.RestartPolicy.MaximumRetryCount == 0 && host.PidMode == "" && host.UTSMode == "" &&
		len(host.Binds) == 0 && len(host.Mounts) == 0 && len(host.VolumesFrom) == 0 && len(host.Devices) == 0 &&
		len(host.DeviceRequests) == 0 && len(host.PortBindings) == 0 && len(host.Links) == 0 && len(host.ExtraHosts) == 0 &&
		len(host.CapAdd) == 0 && len(host.CapDrop) == 1 && host.CapDrop[0] == "ALL" &&
		len(host.SecurityOpt) == 1 && host.SecurityOpt[0] == "no-new-privileges:true" &&
		host.LogConfig.Type == "none" && host.Memory == fenceMemory && host.NanoCPUs == fenceCPU &&
		host.PidsLimit != nil && *host.PidsLimit == 1
}

// fenceInventory corroborates the shared admission ledger's permanent slot
// limit. It includes every instance's markers; a foreign marker is never ours
// to delete. Docker's bounded list is deliberately one larger than the quota.
func (b *Backend) fenceInventory(ctx context.Context, replacing string) error {
	rows, err := b.api.ContainerList(ctx, container.ListOptions{All: true, Limit: backend.MaxLaunchFences + 1,
		Filters: filters.NewArgs(filters.Arg("label", backend.LabelLaunchFence))})
	if err != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	seen := map[string]bool{}
	replaces := false
	for _, row := range rows {
		if row.ID == "" || seen[row.ID] {
			return backend.ErrLaunchFenceOwnership
		}
		if _, ok := row.Labels[backend.LabelLaunchFence]; !ok {
			return backend.ErrLaunchFenceOwnership
		}
		seen[row.ID] = true
		replaces = replaces || row.ID == replacing
	}
	if len(rows) > backend.MaxLaunchFences || len(rows) == backend.MaxLaunchFences && !replaces {
		return fmt.Errorf("the permanent Docker recovery marker limit has been reached: %w", backend.ErrLaunchFencePending)
	}
	return nil
}

// FenceLaunch never adopts an unacknowledged Create. The API must reuse a saved
// acknowledged proof via VerifyLaunchFence; otherwise an exact owned scratch
// object is removed by ID and a new, acknowledged one is created.
func (b *Backend) FenceLaunch(ctx context.Context, req backend.LaunchFenceRequest) (backend.LaunchFenceProof, error) {
	if _, err := b.fenceRequestIdentity(ctx, req); err != nil {
		return backend.LaunchFenceProof{}, err
	}
	if _, active := b.creates.Load(req.SandboxID); active {
		return backend.LaunchFenceProof{}, backend.ErrLaunchFencePending
	}
	name := namePrefix + req.SandboxID
	original, err := b.api.ContainerInspect(ctx, name)
	missing := err != nil && ctx.Err() == nil && stoppedCleanupNotFound(err)
	if err != nil && !missing {
		return backend.LaunchFenceProof{}, backend.ErrUnavailable
	}
	replacing := ""
	if !missing {
		if fenceMarked(original) {
			if !validFenceContainer(req, original) {
				return backend.LaunchFenceProof{}, backend.ErrLaunchFenceOwnership
			}
			replacing = original.ID
		} else if !b.ownedFenceOriginal(req, original) {
			return backend.LaunchFenceProof{}, backend.ErrLaunchFenceOwnership
		}
	}
	if err = b.fenceInventory(ctx, replacing); err != nil {
		return backend.LaunchFenceProof{}, err
	}
	allowedRef := ""
	if !missing {
		allowedRef = original.ID
	}
	if err = b.fenceAttemptInventory(ctx, req, allowedRef); err != nil {
		return backend.LaunchFenceProof{}, err
	}
	if !missing {
		fresh, observeErr := b.api.ContainerInspect(ctx, original.ID)
		if observeErr != nil || fresh.ID != original.ID ||
			(replacing != "" && !validFenceContainer(req, fresh)) ||
			(replacing == "" && !b.ownedFenceOriginal(req, fresh)) {
			return backend.LaunchFenceProof{}, backend.ErrLaunchFenceOwnership
		}
		// A confirmed original may still be running: stopping precisely that
		// child is part of this separately acknowledged operator action. An
		// unacknowledged inert fence never needs force.
		if err = b.api.ContainerRemove(ctx, original.ID, container.RemoveOptions{Force: replacing == "", RemoveVolumes: true}); err != nil {
			return backend.LaunchFenceProof{}, fmt.Errorf("remove exact interrupted allocation: %w", backend.ErrLaunchFencePending)
		}
	}
	if _, err = b.fenceRequestIdentity(ctx, req); err != nil {
		return backend.LaunchFenceProof{}, err
	}
	cfg, host := fenceConfiguration(req)
	created, err := b.api.ContainerCreate(ctx, cfg, host, nil, nil, name)
	if err != nil || ctx.Err() != nil || !fenceHex(created.ID, 32) {
		// Neither absence nor a name conflict nor inspection of a possible
		// result converts an uncertain acknowledgment into durable proof.
		return backend.LaunchFenceProof{}, backend.ErrLaunchFencePending
	}
	ack := time.Now().UTC().Format(time.RFC3339Nano)
	proof := backend.LaunchFenceProof{Protocol: backend.LaunchFenceProtocol, EngineID: req.EngineID,
		InstanceID: req.InstanceID, SandboxID: req.SandboxID, SessionID: req.SessionID, IntentID: req.IntentID,
		ScopeDigest: req.ScopeDigest, OperationDigest: req.OperationDigest, FenceRef: created.ID,
		ConfigDigest: fenceConfigurationDigest(req), AcknowledgedAt: ack, ObservedAt: ack}
	if err = b.VerifyLaunchFence(ctx, req, proof); err != nil {
		return backend.LaunchFenceProof{}, err
	}
	proof.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return proof, nil
}

func (b *Backend) ownedFenceOriginal(req backend.LaunchFenceRequest, resp container.InspectResponse) bool {
	return resp.ContainerJSONBase != nil && fenceHex(resp.ID, 32) && resp.Name == "/"+namePrefix+req.SandboxID &&
		resp.Config != nil && resp.State != nil && !fenceMarked(resp) &&
		(req.ExpectedOriginalRef == "" || req.ExpectedOriginalRef == resp.ID) &&
		resp.Config.Labels[backend.LabelManaged] == "true" && resp.Config.Labels[backend.LabelInstance] == b.instance &&
		resp.Config.Labels[backend.LabelSandboxID] == req.SandboxID && resp.Config.Labels[backend.LabelSessionID] == req.SessionID
}

func (b *Backend) VerifyLaunchFence(ctx context.Context, req backend.LaunchFenceRequest, proof backend.LaunchFenceProof) error {
	if _, err := b.fenceRequestIdentity(ctx, req); err != nil {
		return err
	}
	ack, e1 := time.Parse(time.RFC3339Nano, proof.AcknowledgedAt)
	observed, e2 := time.Parse(time.RFC3339Nano, proof.ObservedAt)
	if proof.Protocol != backend.LaunchFenceProtocol || proof.EngineID != req.EngineID || proof.InstanceID != req.InstanceID ||
		proof.SandboxID != req.SandboxID || proof.SessionID != req.SessionID || proof.IntentID != req.IntentID ||
		proof.ScopeDigest != req.ScopeDigest || proof.OperationDigest != req.OperationDigest ||
		proof.ConfigDigest != fenceConfigurationDigest(req) || !fenceHex(proof.FenceRef, 32) ||
		e1 != nil || e2 != nil || ack.IsZero() || observed.Before(ack) || observed.After(time.Now().UTC()) {
		return backend.ErrLaunchFenceOwnership
	}
	resp, err := b.api.ContainerInspect(ctx, proof.FenceRef)
	if err != nil || resp.ID != proof.FenceRef || !validFenceContainer(req, resp) {
		return backend.ErrLaunchFenceOwnership
	}
	// Verify both the immutable object and its name claim. A renamed object is
	// not a fence, even if its labels and private proof were retained.
	byName, err := b.api.ContainerInspect(ctx, namePrefix+req.SandboxID)
	if err != nil || byName.ID != proof.FenceRef || !validFenceContainer(req, byName) {
		return backend.ErrLaunchFenceOwnership
	}
	if err = b.fenceAttemptInventory(ctx, req, proof.FenceRef); err != nil {
		return err
	}
	_, err = b.fenceRequestIdentity(ctx, req)
	return err
}

// A name claim prevents future Creates but cannot explain away an existing
// original which somebody renamed. Require the entire exact-child inventory
// to contain only the object already inspected (or nothing before creation).
// Two matching rows suffice to refuse; no large history scan is needed.
func (b *Backend) fenceAttemptInventory(ctx context.Context, req backend.LaunchFenceRequest, allowedRef string) error {
	rows, err := b.api.ContainerList(ctx, container.ListOptions{All: true, Limit: 2, Filters: filters.NewArgs(
		filters.Arg("label", backend.LabelInstance+"="+req.InstanceID),
		filters.Arg("label", backend.LabelSandboxID+"="+req.SandboxID),
		filters.Arg("label", backend.LabelSessionID+"="+req.SessionID))})
	if err != nil || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	if allowedRef == "" && len(rows) != 0 || allowedRef != "" && (len(rows) != 1 || rows[0].ID != allowedRef) {
		return backend.ErrLaunchFenceOwnership
	}
	for _, row := range rows {
		if row.Labels[backend.LabelInstance] != req.InstanceID || row.Labels[backend.LabelSandboxID] != req.SandboxID || row.Labels[backend.LabelSessionID] != req.SessionID {
			return backend.ErrLaunchFenceOwnership
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
