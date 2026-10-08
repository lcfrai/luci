package main

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type launchFenceBackendFixture struct {
	*availableBackend
	req    backend.LaunchFenceRequest
	proof  backend.LaunchFenceProof
	ctx    context.Context
	result error
}

func (b *launchFenceBackendFixture) LaunchFenceIdentity(ctx context.Context) (backend.LaunchFenceIdentity, error) {
	b.ctx = ctx
	return backend.LaunchFenceIdentity{Protocol: 1, EngineID: "engine", InstanceID: "instance"}, b.result
}
func (b *launchFenceBackendFixture) FenceLaunch(ctx context.Context, req backend.LaunchFenceRequest) (backend.LaunchFenceProof, error) {
	b.ctx, b.req = ctx, req
	return b.proof, b.result
}
func (b *launchFenceBackendFixture) VerifyLaunchFence(ctx context.Context, req backend.LaunchFenceRequest, proof backend.LaunchFenceProof) error {
	b.ctx, b.req, b.proof = ctx, req, proof
	return b.result
}
func TestRecoveringBackendLaunchFencePreservesExactProofAndRefusesFallback(t *testing.T) {
	ctx := context.Background()
	req := backend.LaunchFenceRequest{EngineID: "engine", InstanceID: "instance", SandboxID: "child", IntentID: "intent", ExpectedOriginalRef: "immutable-original"}
	proof := backend.LaunchFenceProof{FenceRef: "immutable-fence", ConfigDigest: "config"}
	fixture := &launchFenceBackendFixture{availableBackend: &availableBackend{}, proof: proof}
	b := &recoveringBackend{factory: func() (backend.Backend, error) { return fixture, nil }}
	if identity, err := b.LaunchFenceIdentity(ctx); err != nil || identity.Protocol != 1 {
		t.Fatal(identity, err)
	}
	if got, err := b.FenceLaunch(ctx, req); err != nil || got != proof || fixture.req != req || fixture.ctx != ctx {
		t.Fatal(got, err)
	}
	if err := b.VerifyLaunchFence(ctx, req, proof); err != nil || fixture.proof != proof {
		t.Fatal(err)
	}
	fixture.result = backend.ErrLaunchFencePending
	if _, err := b.FenceLaunch(ctx, req); !errors.Is(err, backend.ErrLaunchFencePending) {
		t.Fatal(err)
	}
	legacy := &availableBackend{}
	b = &recoveringBackend{factory: func() (backend.Backend, error) { return legacy, nil }}
	if _, err := b.LaunchFenceIdentity(ctx); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := b.FenceLaunch(ctx, req); !errors.Is(err, backend.ErrUnsupported) || legacy.destroyedID != "" {
		t.Fatal("fence fell back to ordinary Destroy", err)
	}
	if err := b.VerifyLaunchFence(ctx, req, proof); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatal(err)
	}
	b = &recoveringBackend{factory: func() (backend.Backend, error) { return nil, backend.ErrUnavailable }}
	if _, err := b.FenceLaunch(ctx, req); !errors.Is(err, backend.ErrUnavailable) {
		t.Fatal(err)
	}
}
