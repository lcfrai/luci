package backend

import "context"

// Directory custody only. This neither authorizes a workspace/run nor proves
// emptiness, sibling exclusion, payload hashes or native borrower settlement.
// The sole API owner retains its acquisition/registry lease across transfers,
// latches retirement independently, and waits for its own borrowers before
// explicit Close/Transfer. No original native handle or payload is exported.
type DocumentImageAssemblyNamespaceGuard interface {
	CheckCurrent(context.Context) error
	Transfer(context.Context) (DocumentImageAssemblyNamespaceGuard, error)
	Close() error
}
