package main

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func (b *recoveringBackend) checkExecutor() (backend.CheckExecutor, error) {
	value, err := b.get()
	if err != nil {
		return nil, err
	}
	executor, ok := value.(backend.CheckExecutor)
	if !ok {
		return nil, backend.ErrUnsupported
	}
	return executor, nil
}
func (b *recoveringBackend) PrepareCheck(ctx context.Context, spec backend.CheckSpec) (backend.PreparedCheck, error) {
	v, e := b.checkExecutor()
	if e != nil {
		return backend.PreparedCheck{}, e
	}
	return v.PrepareCheck(ctx, spec)
}
func (b *recoveringBackend) CreateCheck(ctx context.Context, p backend.PreparedCheck) (string, error) {
	v, e := b.checkExecutor()
	if e != nil {
		return "", e
	}
	return v.CreateCheck(ctx, p)
}
func (b *recoveringBackend) StartCheck(ctx context.Context, id backend.CheckIdentity, ref string) error {
	v, e := b.checkExecutor()
	if e != nil {
		return e
	}
	return v.StartCheck(ctx, id, ref)
}
func (b *recoveringBackend) InspectCheck(ctx context.Context, id backend.CheckIdentity, ref string) (backend.CheckObservation, error) {
	v, e := b.checkExecutor()
	if e != nil {
		return backend.CheckObservation{}, e
	}
	return v.InspectCheck(ctx, id, ref)
}
func (b *recoveringBackend) CheckLogs(ctx context.Context, id backend.CheckIdentity, ref string, limit int) (backend.CheckOutput, error) {
	v, e := b.checkExecutor()
	if e != nil {
		return backend.CheckOutput{}, e
	}
	return v.CheckLogs(ctx, id, ref, limit)
}
func (b *recoveringBackend) RemoveCheck(ctx context.Context, id backend.CheckIdentity, ref string, force bool) error {
	v, e := b.checkExecutor()
	if e != nil {
		return e
	}
	return v.RemoveCheck(ctx, id, ref, force)
}
func (b *recoveringBackend) FenceCheckLaunch(ctx context.Context, req backend.CheckFenceRequest) (backend.CheckFenceProof, error) {
	v, e := b.checkExecutor()
	if e != nil {
		return backend.CheckFenceProof{}, e
	}
	return v.FenceCheckLaunch(ctx, req)
}
func (b *recoveringBackend) VerifyCheckLaunchFence(ctx context.Context, req backend.CheckFenceRequest, p backend.CheckFenceProof) error {
	v, e := b.checkExecutor()
	if e != nil {
		return e
	}
	return v.VerifyCheckLaunchFence(ctx, req, p)
}
func (b *recoveringBackend) ValidateResourceLimits(ctx context.Context, r backend.Resources) error {
	value, e := b.get()
	if e != nil {
		return e
	}
	v, ok := value.(backend.ResourceLimitValidator)
	if !ok {
		return backend.ErrUnsupported
	}
	return v.ValidateResourceLimits(ctx, r)
}
