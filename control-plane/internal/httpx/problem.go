// Package httpx holds the two things every control-plane handler needs and that
// belong to no single handler: the one error body the whole API answers with,
// and the guard deciding whether a browser context may talk to this process.
//
// It imports only the standard library, and must keep doing so. Tasks 10 to 13
// all import it; an edge pointing the other way becomes an import cycle the
// first time a handler needs a helper.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MaxBodyBytes bounds a request body. The REST surface carries an opening
// prompt, not a file.
const MaxBodyBytes = 1 << 20

// Problem is the error body of every non-2xx JSON response on this API. One
// shape everywhere means the desktop's client has one error branch.
type Problem struct {
	// Error is a stable machine-readable code. The desktop switches on it.
	Error string `json:"error"`
	// Message is one human sentence, safe to render. It never contains a token,
	// a file path outside the user's own projects, or a Go error chain.
	Message string `json:"message"`
	// Details carries structured context, e.g. {"id": "s-3f2a91c0"}.
	Details map[string]string `json:"details,omitempty"`
}

// The complete set of error codes: contract 3.2's fifteen, plus the
// non_loopback_publish that Group C's shared-package ruling 7 adds. That one is
// task 12's, and it is a sixteenth code rather than a reuse of
// sandbox_not_running because a sandbox the daemon reports published off
// loopback IS running - saying otherwise would be a lie the cockpit then shows
// the user. They are constants and not string literals at the call sites
// because the desktop switches on them: a typo in one handler is a code the
// cockpit can never match and that no compiler would have flagged.
const (
	CodeInvalidJSON        = "invalid_json"         // 400 not JSON, unknown field, or over 1 MiB
	CodeBadRequest         = "bad_request"          // 400 a field is missing or unusable
	CodeUnknownHarness     = "unknown_harness"      // 400 harness is not in the catalogue
	CodeUnsupportedOption  = "unsupported_option"   // 400 the harness does not accept that option
	CodeForbiddenOrigin    = "forbidden_origin"     // 403 Origin present and not allowlisted
	CodeForbiddenHost      = "forbidden_host"       // 403 Host is not loopback
	CodeProjectNotFound    = "project_not_found"    // 404
	CodeWorktreeNotFound   = "worktree_not_found"   // 404
	CodeSessionNotFound    = "session_not_found"    // 404
	CodeConflict           = "conflict"             // 409 wrong state for the verb
	CodeSandboxNotRunning  = "sandbox_not_running"  // 409 attach or restart, sandbox gone
	CodeHarnessUnavailable = "harness_unavailable"  // 409 in the catalogue, not in the image
	CodeUpgradeRequired    = "upgrade_required"     // 426 /attach without WebSocket headers
	CodeInternal           = "internal"             // 500 message is generic, detail goes to the log
	CodeNonLoopbackPublish = "non_loopback_publish" // 500 sandbox port bound off loopback (task 12)
	CodeBackendUnavailable = "backend_unavailable"  // 503 no Docker daemon, no kubeconfig
)

// internalBody is the one response this package can always send. It is a
// constant so the encode-failure path cannot itself fail to encode.
const internalBody = `{"error":"internal","message":"The control plane failed to encode its own response."}` + "\n"

func writeBytes(w http.ResponseWriter, status int, body []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	// The Message can carry text the caller supplied. A browser that sniffs a
	// JSON error body into HTML would make that an XSS.
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteJSON writes v as JSON with the given status.
//
// It marshals into a buffer BEFORE touching w. Encoding straight to the
// ResponseWriter would send the status line first, so a value that fails
// halfway through produces a 200 with a truncated body and no way back. Buffered,
// the failure is still a truthful 500.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		writeBytes(w, http.StatusInternalServerError, []byte(internalBody))
		return
	}
	writeBytes(w, status, buf.Bytes())
}

// WriteProblem writes a Problem.
func WriteProblem(w http.ResponseWriter, status int, code, msg string, details map[string]string) {
	WriteJSON(w, status, Problem{Error: code, Message: msg, Details: details})
}

// DecodeJSON reads a JSON body into dst. It rejects unknown fields and bodies
// over 1 MiB, writes a 400 invalid_json Problem itself on failure, and reports
// whether the handler should continue.
//
// It deliberately does NOT require Content-Type: application/json. Requiring it
// would force a preflight on cross-origin posts and so add a second CSRF
// defence, but OriginPolicy already rejects every cross-origin caller, and the
// requirement breaks curl for nothing.
func DecodeJSON[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		WriteProblem(w, http.StatusBadRequest, CodeInvalidJSON, decodeMessage(err), nil)
		return false
	}
	// A second value in the body is not documented anywhere on this API, and
	// the silent reading is the dangerous one: `{"a":1}{"b":2}` otherwise
	// decodes as the first object with the second dropped and no complaint.
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		// This branch also catches an over-limit body whose FIRST value parsed
		// - `{"name":"a"}` followed by a megabyte of padding - where the body is
		// not two objects and blaming one would name the wrong fault. Only the
		// size error is surfaced. Every other error keeps the plain sentence,
		// because DisallowUnknownFields is still set on this decoder: a genuine
		// second object makes Decode complain about an unknown field on the
		// empty struct, which describes this package's own probe rather than
		// anything the caller did.
		msg := "Body must contain exactly one JSON object."
		if errors.As(err, new(*http.MaxBytesError)) {
			msg = decodeMessage(err)
		}
		WriteProblem(w, http.StatusBadRequest, CodeInvalidJSON, msg, nil)
		return false
	}
	return true
}

// decodeMessage turns a decoder error into one human sentence.
//
// The json package's own text for a syntax, type or unknown-field error is
// reused, because it names the offending field or byte offset and carries no
// secret and no filesystem path. It does reveal Go field names on a type error;
// on a loopback developer tool that is worth the diagnosability. Anything this
// does not recognise gets a generic sentence rather than err.Error(), because
// that is the branch an unexpected wrapped error takes and contract 2.12
// forbids a Go error chain in Message.
func decodeMessage(err error) string {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return fmt.Sprintf("Request body exceeds the %d byte limit.", MaxBodyBytes)
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("Body is not valid JSON: %s (at byte %d).", syntax, syntax.Offset)
	}
	var wrongType *json.UnmarshalTypeError
	if errors.As(err, &wrongType) {
		if wrongType.Field != "" {
			return fmt.Sprintf("Field %q must be %s, not %s.", wrongType.Field, wrongType.Type, wrongType.Value)
		}
		return fmt.Sprintf("Body must be %s, not %s.", wrongType.Type, wrongType.Value)
	}
	switch {
	case errors.Is(err, io.EOF):
		return "Body is empty; a JSON object is required."
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "Body ended in the middle of a JSON value."
	}
	if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return "Body has an unknown field: " + field + "."
	}
	return "Body could not be read as JSON."
}
