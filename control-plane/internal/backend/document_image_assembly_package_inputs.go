package backend

import "context"

const (
	DocumentImageAssemblyPackageInputCount                  = 174
	DocumentImageAssemblyPackageInputBytes            int64 = 144257468
	DocumentImageAssemblyPackageInputMaxObjectBytes   int64 = 64 << 20
	DocumentImageAssemblyPackageInputMaxNativeHandles       = 1024
	DocumentImageAssemblyPackageInputMaxTransfers           = 256
)

// Read-free ownership of one exact declared package set. Object metadata is
// containment, not tuple authentication, a payload hash observation or trust.
// Reuses EvidenceObject with fixed package.000..package.173 ordinal roles. The
// private installer must resolve paths and compare the complete pinned tuples.
// No ordinary-file path, numeric HANDLE or JSON can mint a producer completion.
type DocumentImageAssemblyPackageInputGuard interface {
	CheckCurrent(context.Context) error
	Transfer(context.Context) (DocumentImageAssemblyPackageInputGuard, error)
	Close() error
}
