//go:build !windows

package dockerbe

import "github.com/lcfrai/luci/control-plane/internal/backend"

func validateAssemblyNamespaceDirectory(string) error { return backend.ErrUnsupported }
