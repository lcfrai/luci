package api

// RegisteredPatterns returns every ServeMux pattern New registered, across the
// guarded mux and the outer one, guarded first.
//
// It exists for the route audit Group C ruling 1 requires: contract §10a says no
// GET may change state, net/http offers no way to enumerate a mux, and an
// invariant checked against a hand-maintained list is an invariant that stops
// being checked the day someone forgets the list. Registration records instead —
// see routes in api.go — so the audit walks what the product actually serves,
// including the routes tasks 11 and 12 add.
//
// It reads both recorders AT CALL TIME. The first version of this file copied
// their contents into a field partway through New, which silently excused every
// route registered after that line — including, had it survived, the ones tasks
// 11 and 12 add at the bottom of the function.
//
// It lives in export_test.go rather than in api.go because nothing in the
// product needs it, and a route table the package exports is a route table
// something eventually branches on.
func (a *API) RegisteredPatterns() []string {
	out := make([]string, 0, len(a.guarded.registered)+len(a.root.registered))
	out = append(out, a.guarded.registered...)
	out = append(out, a.root.registered...)
	return out
}
