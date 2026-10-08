package main

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func (b *recoveringBackend) launchFencer() (backend.LaunchFencer, error) {
	value, err := b.get()
	if err != nil {
		return nil, err
	}
	fencer, ok := value.(backend.LaunchFencer)
	if !ok {
		return nil, backend.ErrUnsupported
	}
	return fencer, nil
}

func (b *recoveringBackend) LaunchFenceIdentity(ctx context.Context) (backend.LaunchFenceIdentity, error) {
	fencer, err := b.launchFencer()
	if err != nil {
		return backend.LaunchFenceIdentity{}, err
	}
	return fencer.LaunchFenceIdentity(ctx)
}

func (b *recoveringBackend) FenceLaunch(ctx context.Context, req backend.LaunchFenceRequest) (backend.LaunchFenceProof, error) {
	fencer, err := b.launchFencer()
	if err != nil {
		return backend.LaunchFenceProof{}, err
	}
	return fencer.FenceLaunch(ctx, req)
}

func (b *recoveringBackend) VerifyLaunchFence(ctx context.Context, req backend.LaunchFenceRequest, proof backend.LaunchFenceProof) error {
	fencer, err := b.launchFencer()
	if err != nil {
		return err
	}
	return fencer.VerifyLaunchFence(ctx, req, proof)
}
