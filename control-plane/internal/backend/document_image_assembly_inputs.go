package backend

import "context"

// Server-owned retained evidence/source objects, never HTTP paths or mounts.
// Opening guards pins identity; the actual typed producer must verify each hash
// under those guards before adoption. Syntax and CheckCurrent are not that proof.
type DocumentImageAssemblyRetainedObject struct {
	Path   string
	Bytes  int64
	Digest string
}

// Borrowed view only. The dedicated assembler owns all handles and closure.
// Views cannot outlive ownership or be restored from durable receipt metadata.
type DocumentImageAssemblyInputGuard interface {
	CheckCurrent(context.Context) error
}
