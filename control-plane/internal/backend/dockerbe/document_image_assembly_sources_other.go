//go:build !windows

package dockerbe

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type assemblySourceGuard struct{}

func assemblySourceGuardSupported() bool { return false }

func (*assemblySourceGuard) Close() error                       { return nil }
func (*assemblySourceGuard) CheckCurrent(context.Context) error { return backend.ErrUnsupported }
func lockAssemblySources(context.Context, backend.DocumentImageAssemblySpec, ...backend.DocumentImageAssemblyRetainedObject) (*assemblySourceGuard, error) {
	// An advisory flock is not equivalent to mandatory no-write/delete sharing.
	// No silent host-platform weakening is allowed for this optional profile.
	return nil, backend.ErrUnsupported
}
