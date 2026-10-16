package backend

import "context"

const (
	DocumentImageAssemblyEvidenceMaxObjects     = 256
	DocumentImageAssemblyEvidenceMaxObjectBytes = 16 << 20
	DocumentImageAssemblyEvidenceMaxBytes       = 64 << 20
	DocumentImageAssemblyEvidenceMaxReads       = 1024
)

// Exact installer-owned original objects. Role is a unique logical artifact key,
// not a path, URL or accepted evidence claim. No assembly allocation is implied.
type DocumentImageAssemblyEvidenceObject struct {
	Role   string
	Path   string
	Bytes  int64
	Digest string
}

// A single owning token, not a durable capability reconstructed from metadata.
// Transfer moves ownership without duplicating/reopening original native handles.
// A partial acquisition returned with error remains owned and must be closed.
type DocumentImageAssemblyEvidenceGuard interface {
	CheckCurrent(context.Context) error
	ReadBounded(context.Context, string, int64) ([]byte, error)
	Transfer(context.Context) (DocumentImageAssemblyEvidenceGuard, error)
	Close() error
}
