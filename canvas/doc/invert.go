package doc

// The inverse of a batch is computed inside Prepare, op by op, from the state
// each op replaced — never from the op alone, because the op does not know what
// it overwrote. That is what lets per-author undo and revert-by-session (task
// 12) work without a second history.
//
//	create_node  → delete_node
//	delete_node  → create_node × the subtree, pre-order (so each parent exists
//	               before its children, which is why a batch sees earlier ops)
//	set_props    → set_props of exactly the fields and keys it named, with the
//	               old values, and nil where the key was absent
//	move         → move to the old parent and order
//	create_page  → delete_page
//	delete_page  → create_page, then create_node × each artboard subtree
//	update_page  → update_page of exactly the fields it named, with the old values
//	set_token    → set_token of the old token, or delete_token when there was none
//	delete_token → set_token of the deleted token
//	add_asset, the thread ops, the review ops, review_mark → no inverse
//
// Change.Inverse is the concatenation of those groups in REVERSE op order, so
// applying it with Apply restores the content of the prior revision. Each group
// keeps its own order: a delete_node's creates must still run parents first.
//
// The inverse restores content, not provenance. Applying it is a new revision
// by whoever applied it, so Revision and EditedBy move forward on every node it
// touches — an undo is an edit, and the ledger says who made it.

// Invertible reports whether an op kind has an inverse. Exactly the content
// kinds do: a comment, a review, an asset and a review_mark are not design
// content, are never undone, and do not make a batch a content change.
func Invertible(k OpKind) bool {
	switch k {
	case OpCreatePage, OpUpdatePage, OpDeletePage,
		OpCreateNode, OpSetProps, OpMove, OpDeleteNode,
		OpSetToken, OpDeleteToken:
		return true
	}
	return false
}
