package api

// The connection surface: the one route a Panacea key enters by, and the three
// that report or end what it bought. Nothing here ever writes the key back out.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// connectionBody is the wire shape of a status, and the whole of it. It has no
// token field, no prefix of one and no length of one, and the test that says so
// looks for the key's own bytes in the response rather than for a field name -
// a struct that grew one would have to be caught by what it emits.
//
// The nullable fields are pointers rather than empty strings because the
// cockpit draws "never checked" differently from "checked and empty", and
// omitempty would have made the two indistinguishable.
type connectionBody struct {
	State        string       `json:"state"`
	Origin       *string      `json:"origin"`
	OriginLocked bool         `json:"originLocked"`
	APIVersion   *string      `json:"apiVersion"`
	Account      *accountBody `json:"account"`
	Message      string       `json:"message"`
	CheckedAt    *string      `json:"checkedAt"`
}

// accountBody is who Panacea says the key belongs to. Approved is deliberately
// NOT here: the STATE carries it (awaiting_approval), and two spellings of one
// fact drift the first time one of them is updated alone.
type accountBody struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Admin    bool   `json:"admin"`
}

// connectionMillis is the wire's instant: RFC 3339, UTC, milliseconds. The
// cockpit renders it, and a format that varies its precision by the value makes
// a column of them jump.
const connectionMillis = "2006-01-02T15:04:05.000Z07:00"

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func connectionBodyOf(s connection.Status) connectionBody {
	body := connectionBody{
		State:        string(s.State),
		Origin:       nilIfEmpty(s.Origin),
		OriginLocked: s.OriginLocked,
		APIVersion:   nilIfEmpty(s.APIVersion),
		Message:      s.Message,
	}
	if s.Account != nil {
		body.Account = &accountBody{
			Username: s.Account.Username,
			Email:    s.Account.Email,
			Admin:    s.Account.Admin,
		}
	}
	if !s.CheckedAt.IsZero() {
		at := s.CheckedAt.UTC().Format(connectionMillis)
		body.CheckedAt = &at
	}
	return body
}

// connectRequest is the PUT body: an address and a key.
type connectRequest struct {
	Origin string        `json:"origin"`
	Token  redactedToken `json:"token"`
}

// redactedToken is a string that refuses to print itself. The key is decoded
// into one so that a %v added to a log line later - in a handler, in a
// middleware, in a panic's own dump of the request struct - cannot print it.
// The type is the defence; a rule about never logging the field is not.
type redactedToken string

const redactedTokenText = "[redacted]"

func (redactedToken) String() string   { return redactedTokenText }
func (redactedToken) GoString() string { return `"` + redactedTokenText + `"` }

// The three local refusals, each a sentence a user can act on without leaving
// the cockpit.
const (
	connectNeedsToken  = "Paste your Panacea API key into the key field."
	connectNeedsOrigin = "Enter the address of your Panacea, e.g. https://dev.panacea.kpmg."
	connectNeedsPrefix = "A Panacea API key starts with pnc_. Copy it from the portal: Account → API keys."
)

func (a *API) deployConnectionRoutes() []route {
	return []route{
		{"GET /v1/deploy/connection", http.HandlerFunc(a.handleDeployConnection)},
		{"PUT /v1/deploy/connection", http.HandlerFunc(a.handleDeployConnect)},
		{"DELETE /v1/deploy/connection", http.HandlerFunc(a.handleDeployDisconnect)},
		{"POST /v1/deploy/connection/check", http.HandlerFunc(a.handleDeployConnectionCheck)},
	}
}

// registerDeployConnection mounts them on the guarded mux. api.New calls it
// unconditionally: the routes exist even with no Manager, because a surface
// that appears only when configured answers 404 for "not configured", and a 404
// tells the cockpit the control plane is too old rather than that it has no
// Panacea.
func (a *API) registerDeployConnection(mux *routes) {
	for _, rt := range a.deployConnectionRoutes() {
		mux.Handle(rt.Pattern, rt.Handler)
	}
}

// deployConnection is the guard every phase 7 route starts with: the Manager,
// or the one 409 the code deploy_not_connected exists for. It answers through
// writeDeployError with deploy.ErrNotConnected rather than writing its own
// problem, so the sentence and the details cannot drift from the mapping.
//
// Nil happens only in tests: a connection.New failure is fatal at startup.
func (a *API) deployConnection(w http.ResponseWriter, r *http.Request, op string) (*connection.Manager, bool) {
	if a.deps.DeployConnection == nil {
		a.writeDeployError(w, r, op, deploy.ErrNotConnected)
		return nil, false
	}
	return a.deps.DeployConnection, true
}

// handleDeployConnection answers the cached status. It makes no network call:
// contract 10a's audit lists this route as read-only, and a GET that probed
// Panacea would be a side effect a hostile page could trigger with an <img>.
func (a *API) handleDeployConnection(w http.ResponseWriter, r *http.Request) {
	m, ok := a.deployConnection(w, r, "deploy connection")
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, connectionBodyOf(m.Status()))
}

// handleDeployConnect is the one route a Panacea key enters by.
//
// The three refusals below happen BEFORE Connect, so a key with the wrong shape
// is never sent anywhere, and an address is never contacted without one.
func (a *API) handleDeployConnect(w http.ResponseWriter, r *http.Request) {
	m, ok := a.deployConnection(w, r, "deploy connect")
	if !ok {
		return
	}
	var req connectRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	token := strings.TrimSpace(string(req.Token))
	origin := strings.TrimSpace(req.Origin)

	switch {
	case token == "":
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, connectNeedsToken, nil)
		return
	case !strings.HasPrefix(token, "pnc_"):
		// A paste from the wrong field is the common mistake, and the wrong
		// field on that page holds a bearer token for something else. Refusing
		// it here means it is never sent to Panacea and never stored.
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, connectNeedsPrefix, nil)
		return
	case origin == "" && !m.Status().OriginLocked:
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, connectNeedsOrigin, nil)
		return
	}

	status, err := m.Connect(r.Context(), origin, token)
	switch {
	case errors.Is(err, connection.ErrOriginLocked):
		// 400, not the 422 writeDeployError would make of it: Panacea refused
		// nothing. The caller sent an address an administrator had already
		// decided, and the wire lists this as bad_request.
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, connection.OriginLockedMessage, nil)
		return
	case err != nil:
		a.writeDeployError(w, r, "deploy connect", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, connectionBodyOf(status))
}

// handleDeployConnectionCheck re-proves the stored key. Check folds every
// failure into the status it returns - that IS the answer to "does this key
// still work" - so this route has no error branch of its own.
func (a *API) handleDeployConnectionCheck(w http.ResponseWriter, r *http.Request) {
	m, ok := a.deployConnection(w, r, "deploy connection check")
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, connectionBodyOf(m.Check(r.Context())))
}

// handleDeployDisconnect removes the key and keeps the address, so reconnecting
// asks for one field rather than two.
func (a *API) handleDeployDisconnect(w http.ResponseWriter, r *http.Request) {
	m, ok := a.deployConnection(w, r, "deploy disconnect")
	if !ok {
		return
	}
	if err := m.Disconnect(r.Context()); err != nil {
		a.writeDeployError(w, r, "deploy disconnect", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
