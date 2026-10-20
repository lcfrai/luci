//go:build !windows

package dockerbe

import (
 "context"
 "github.com/lcfrai/luci/control-plane/internal/backend"
)
func assemblyBinaryOriginalIdentity(context.Context,assemblyBinaryOriginalBinding)(backend.DocumentImageAssemblyBinaryIdentity,error){return backend.DocumentImageAssemblyBinaryIdentity{},backend.ErrUnsupported}
