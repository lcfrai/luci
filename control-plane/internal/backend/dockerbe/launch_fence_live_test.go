package dockerbe

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// This opt-in test touches only newly created, empty-image, nonce-owned test
// objects. Cleanup is a test-harness exemption, not production fence GC.
// It never pulls/builds an image or inspects/removes unrelated containers.
func TestLaunchFenceLiveDockerAcknowledgmentAndLateRequestBoundaries(t *testing.T) {
	if os.Getenv("LUCI_TEST_DOCKER_FENCE") != "1" {
		t.Skip("set LUCI_TEST_DOCKER_FENCE=1 for the explicitly coordinated tiny Docker acceptance run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	nonce := hex.EncodeToString(token[:])
	instance := "fence-test-" + nonce
	live := &liveFenceDockerClient{APIClient: cli, identity: cli}
	b, err := New(Config{Client: live, Instance: instance})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := b.LaunchFenceIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(nonce))
	req := backend.LaunchFenceRequest{EngineID: identity.EngineID, InstanceID: instance,
		SandboxID: "s-fence-" + nonce, SessionID: "s-fence-" + nonce, IntentID: nonce,
		ScopeDigest: hex.EncodeToString(digest[:]), OperationDigest: hex.EncodeToString(digest[:])}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		for _, made := range live.created {
			observed, inspectErr := cli.ContainerInspect(cleanup, made.id)
			if stoppedCleanupNotFound(inspectErr) {
				continue
			}
			if inspectErr != nil || observed.ContainerJSONBase == nil || observed.ID != made.id ||
				observed.Config == nil || observed.Config.Labels[backend.LabelInstance] != instance ||
				observed.Name != "/"+namePrefix+req.SandboxID || observed.Config.Labels[backend.LabelSessionID] != req.SessionID {
				t.Errorf("retained unconfirmed test object %s; no cleanup: %v", made.id, inspectErr)
				continue
			}
			if cleanupErr := cli.ContainerRemove(cleanup, made.id, container.RemoveOptions{Force: false}); cleanupErr != nil && !stoppedCleanupNotFound(cleanupErr) {
				t.Errorf("exact inert test cleanup %s: %v", made.id, cleanupErr)
			}
		}
	})

	// The interrupted original is also a tiny empty root and cannot execute;
	// this is backend-protocol acceptance, not a provider or user-workload test.
	cfg, host := fenceConfiguration(req)
	cfg.Labels = map[string]string{backend.LabelManaged: "true", backend.LabelInstance: instance,
		backend.LabelSandboxID: req.SandboxID, backend.LabelSessionID: req.SessionID}
	original, err := live.ContainerCreate(ctx, cfg, host, nil, nil, namePrefix+req.SandboxID)
	if err != nil {
		t.Fatal(err)
	}
	req.ExpectedOriginalRef = original.ID
	proof, err := b.FenceLaunch(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if proof.FenceRef == original.ID {
		t.Fatal("fence reused original immutable identity")
	}
	if err = b.VerifyLaunchFence(ctx, req, proof); err != nil {
		t.Fatal(err)
	}
	if err = b.Destroy(ctx, req.SandboxID); !errors.Is(err, backend.ErrLaunchFenced) {
		t.Fatal("ordinary Destroy did not refuse fence", err)
	}
	if _, err = b.Get(ctx, req.SandboxID); !errors.Is(err, backend.ErrLaunchFenced) {
		t.Fatal("fence presented as a worker", err)
	}
	if _, err = live.ContainerCreate(ctx, cfg, host, nil, nil, namePrefix+req.SandboxID); !errdefs.IsConflict(err) {
		t.Fatal("late original Create did not conflict", err)
	}
	if err = cli.ContainerStart(ctx, original.ID, container.StartOptions{}); !stoppedCleanupNotFound(err) {
		t.Fatal("late original Start was not absent", err)
	}
	if err = cli.ContainerRemove(ctx, original.ID, container.RemoveOptions{Force: true}); !stoppedCleanupNotFound(err) {
		t.Fatal("late immutable cleanup did not remain absent", err)
	}
	if err = b.VerifyLaunchFence(ctx, req, proof); err != nil {
		t.Fatal("late original cleanup damaged fence", err)
	}

	// Simulate loss of the SDK Create acknowledgment after the real daemon
	// returned success. Inspect alone must not adopt this next generation.
	if err = cli.ContainerRemove(ctx, proof.FenceRef, container.RemoveOptions{Force: false}); err != nil {
		t.Fatal(err)
	}
	live.loseNextAcknowledgment = true
	if unknown, e := b.FenceLaunch(ctx, req); !errors.Is(e, backend.ErrLaunchFencePending) || unknown.FenceRef != "" {
		t.Fatal("lost acknowledgment became proof", unknown, e)
	}
	unacknowledged := live.created[len(live.created)-1].id
	replacement, err := b.FenceLaunch(ctx, req)
	if err != nil || replacement.FenceRef == unacknowledged {
		t.Fatal("retry adopted unacknowledged fence", replacement, err)
	}
	if err = cli.ContainerRemove(ctx, unacknowledged, container.RemoveOptions{Force: true}); !stoppedCleanupNotFound(err) {
		t.Fatal("old fence cleanup unexpected", err)
	}
	if err = b.VerifyLaunchFence(ctx, req, replacement); err != nil {
		t.Fatal(err)
	}
	t.Logf("real Docker engine=%s original=%s firstFence=%s lostAckFence=%s acknowledgedReplacement=%s; no image import/pull/build or agent start", identity.EngineID, original.ID, proof.FenceRef, unacknowledged, replacement.FenceRef)
}

type liveFenceCreated struct{ id string }
type liveFenceDockerClient struct {
	APIClient
	identity               fenceIdentityClient
	loseNextAcknowledgment bool
	created                []liveFenceCreated
}

func (c *liveFenceDockerClient) DaemonHost() string { return c.identity.DaemonHost() }
func (c *liveFenceDockerClient) Info(ctx context.Context) (system.Info, error) {
	return c.identity.Info(ctx)
}
func (c *liveFenceDockerClient) ContainerCreate(ctx context.Context, cfg *container.Config, host *container.HostConfig, net *network.NetworkingConfig, platform *ocispec.Platform, name string) (container.CreateResponse, error) {
	created, err := c.APIClient.ContainerCreate(ctx, cfg, host, net, platform, name)
	if err == nil && created.ID != "" {
		c.created = append(c.created, liveFenceCreated{id: created.ID})
	}
	if err == nil && c.loseNextAcknowledgment {
		c.loseNextAcknowledgment = false
		return container.CreateResponse{}, fmt.Errorf("controlled lost Create acknowledgment: %w", context.DeadlineExceeded)
	}
	return created, err
}
