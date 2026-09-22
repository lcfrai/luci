package api

// RegisteredPatterns returns every ServeMux pattern New registered, in
// registration order, across both the guarded mux and the outer one.
//
// It exists for the route audit Group C ruling 1 requires: contract §10a says no
// GET may change state, net/http offers no way to enumerate a mux, and an
// invariant checked against a hand-maintained list is an invariant that stops
// being checked the day someone forgets the list. Registration records instead —
// see routes in api.go — so the audit walks what the product actually serves,
// including the routes tasks 11 and 12 add.
//
// It lives in export_test.go rather than in api.go because nothing in the
// product needs it, and a route table the package exports is a route table
// something eventually branches on.
func (a *API) RegisteredPatterns() []string {
	out := make([]string, len(a.patterns))
	copy(out, a.patterns)
	return out
}
