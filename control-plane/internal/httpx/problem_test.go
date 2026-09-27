package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteProblemWritesTheExactBodyStatusAndHeaders(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteProblem(rec, http.StatusNotFound, CodeSessionNotFound,
		"No session with id s-3f2a91c0.", map[string]string{"id": "s-3f2a91c0"})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	// nosniff, because the Message is rendered from data the caller supplied and
	// a browser that content-sniffs a JSON error body into HTML is an XSS.
	if got, want := rec.Header().Get("X-Content-Type-Options"), "nosniff"; got != want {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, want)
	}

	want := `{"error":"session_not_found","message":"No session with id s-3f2a91c0.","details":{"id":"s-3f2a91c0"}}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
	}
}

func TestAllSixteenErrorCodesHaveTheirContractString(t *testing.T) {
	// Contract 3.2's fifteen plus ruling 7's non_loopback_publish. These strings
	// cross a language boundary: task 16's TypeScript union is written by hand
	// against them, so a typo does not fail loudly here - it ships, and produces
	// a cockpit error branch that silently never matches. That is the exact
	// failure the constants exist to prevent, which is why the constants
	// themselves are worth pinning rather than only the four a test happens to
	// use. Constant keys also make a duplicated value a compile error.
	for code, want := range map[string]string{
		CodeInvalidJSON:        "invalid_json",
		CodeBadRequest:         "bad_request",
		CodeUnknownHarness:     "unknown_harness",
		CodeUnsupportedOption:  "unsupported_option",
		CodeForbiddenOrigin:    "forbidden_origin",
		CodeForbiddenHost:      "forbidden_host",
		CodeProjectNotFound:    "project_not_found",
		CodeWorktreeNotFound:   "worktree_not_found",
		CodeSessionNotFound:    "session_not_found",
		CodeConflict:           "conflict",
		CodeSandboxNotRunning:  "sandbox_not_running",
		CodeHarnessUnavailable: "harness_unavailable",
		CodeUpgradeRequired:    "upgrade_required",
		CodeInternal:           "internal",
		CodeNonLoopbackPublish: "non_loopback_publish",
		CodeBackendUnavailable: "backend_unavailable",
	} {
		if code != want {
			t.Errorf("code = %q, want %q", code, want)
		}
	}
}

// TestPhase7ErrorCodesHaveTheirContractString pins the codes phase 7's deploy
// surface adds, for the reason the sixteen above are pinned: the desktop
// switches on the string, so a typo here is a branch that never matches rather
// than a compile error. It is a second test rather than eleven more rows in the
// first, because that one's name states a count contract 3.2 fixed.
func TestPhase7ErrorCodesHaveTheirContractString(t *testing.T) {
	for code, want := range map[string]string{
		CodeUpstreamUnavailable:         "upstream_unavailable",
		CodeDeployNotConnected:          "deploy_not_connected",
		CodeDeployTokenRejected:         "deploy_token_rejected",
		CodeDeployForbidden:             "deploy_forbidden",
		CodeDeployNotFound:              "deploy_not_found",
		CodeDeployConflict:              "deploy_conflict",
		CodeDeployRejected:              "deploy_rejected",
		CodeDeployNotBound:              "deploy_not_bound",
		CodeDeployEnvironmentInvalid:    "deploy_environment_invalid",
		CodeDeployEnvironmentUnreadable: "deploy_environment_unreadable",
		CodeDeployPrecondition:          "deploy_precondition",
	} {
		if code != want {
			t.Errorf("code = %q, want %q", code, want)
		}
	}
}

// TestWriteProblemDetailsCarriesNumbersAndBools is the reason Details is
// map[string]any: the desktop reads details.status === 0 to tell "Panacea never
// answered, so your Ship may have started" from "nothing was sent". A status
// spelled as a string never matches that, and no === would ever fail loudly.
func TestWriteProblemDetailsCarriesNumbersAndBools(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblemDetails(rec, http.StatusServiceUnavailable, CodeUpstreamUnavailable,
		"Panacea did not answer, so the cockpit cannot tell whether it acted.",
		map[string]any{"upstream": "panacea", "status": 0, "reserved": true})

	want := `{"error":"upstream_unavailable","message":"Panacea did not answer, so the cockpit cannot tell whether it acted.",` +
		`"details":{"reserved":true,"status":0,"upstream":"panacea"}}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
	}
}

func TestWriteProblemOmitsEmptyDetails(t *testing.T) {
	// Contract 3.2: "details is omitted when empty." Not null, omitted: the
	// desktop's model types it as optional and null would be a third state.
	for name, details := range map[string]map[string]string{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteProblem(rec, http.StatusForbidden, CodeForbiddenHost,
				"This control plane only answers requests addressed to localhost.", details)

			want := `{"error":"forbidden_host","message":"This control plane only answers requests addressed to localhost."}` + "\n"
			if got := rec.Body.String(); got != want {
				t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
			}
		})
	}
}

func TestWriteJSONReportsItsOwnEncodingFailureAsA500(t *testing.T) {
	rec := httptest.NewRecorder()

	// A channel cannot be marshalled. The point of the test is the STATUS: the
	// encode must fail before anything reaches the ResponseWriter, because once
	// a 200 status line is on the wire no truthful correction is possible.
	WriteJSON(rec, http.StatusOK, map[string]any{"ch": make(chan int)})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"error":"internal"`) {
		t.Errorf("body = %q, want an internal Problem", got)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("the fallback body is not valid JSON: %v", err)
	}
}

// decodeTarget is the shape DecodeJSON is exercised against. It has one string
// and one int so a type error can name a field.
type decodeTarget struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func decodeBody(t *testing.T, body string) (decodeTarget, *httptest.ResponseRecorder, bool) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost,
		"http://127.0.0.1:8730/v1/sessions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	var dst decodeTarget
	ok := DecodeJSON(rec, req, &dst)
	return dst, rec, ok
}

// problemFrom reads back the Problem DecodeJSON wrote, failing the test if what
// it wrote was not one carrying wantCode.
//
// The code is a parameter rather than a constant because DecodeJSON answers with
// two of them and the difference is the point: invalid_json for bytes that are
// not one JSON object, bad_request for a body that parsed and asked for
// something this endpoint does not have.
func problemFrom(t *testing.T, rec *httptest.ResponseRecorder, wantCode string) Problem {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if p.Error != wantCode {
		t.Errorf("error = %q, want %q", p.Error, wantCode)
	}
	if p.Message == "" {
		t.Error("message is empty; a 400 with no sentence is a bug report with no content")
	}
	return p
}

func TestDecodeJSONAcceptsAWellFormedBody(t *testing.T) {
	dst, rec, ok := decodeBody(t, `{"name":"api","count":3}`)

	if !ok {
		t.Fatalf("DecodeJSON rejected a valid body: %s", rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("DecodeJSON wrote %q on success; the handler owns the response", rec.Body.String())
	}
	if dst.Name != "api" || dst.Count != 3 {
		t.Errorf("dst = %+v, want {Name:api Count:3}", dst)
	}
}

func TestDecodeJSONRejectsAnUnknownFieldAsBadRequestAndNotInvalidJson(t *testing.T) {
	// The reason this is a 400 and not a shrug: "worktreedId" silently ignored
	// is a session created against the wrong worktree, which the user then has
	// to notice themselves.
	//
	// And the reason the code is bad_request: Group C §1B. An unknown field is
	// WELL-FORMED JSON that is semantically wrong, so invalid_json would tell
	// the cockpit something untrue — the same objection that refuses
	// sandbox_not_running for a sandbox that is running. It is decided here
	// rather than in each handler because a rule every caller must remember is
	// not a rule; before this, the worktree PATCH answered bad_request from its
	// own local field scan while every DecodeJSON endpoint answered
	// invalid_json, which is two codes for one mistake on one surface.
	//
	// The wrong-type case below reaches the same answer by the same argument,
	// which is §1B as amended after task 11's first round.
	_, rec, ok := decodeBody(t, `{"name":"api","colour":"blue"}`)

	if ok {
		t.Fatal("DecodeJSON accepted an unknown field")
	}
	p := problemFrom(t, rec, CodeBadRequest)
	if !strings.Contains(p.Message, "colour") {
		t.Errorf("message = %q, want it to name the offending field", p.Message)
	}
}

func TestDecodeJSONSeparatesAnUnparseableBodyFromASemanticallyWrongOne(t *testing.T) {
	// The whole of §1B in one table, and the pair is what makes either code
	// mean anything: invalid_json is reserved for bytes that are not one JSON
	// object, so a cockpit seeing it knows the request never parsed, and
	// bad_request means the JSON was fine and this endpoint cannot honour it.
	for name, tc := range map[string]struct {
		body     string
		wantCode string
	}{
		"truncated":       {`{"name":"a"`, CodeInvalidJSON},
		"not json at all": {`name please`, CodeInvalidJSON},
		"two objects":     {`{"name":"a"}{"name":"b"}`, CodeInvalidJSON},
		"empty":           {``, CodeInvalidJSON},
		"unknown field":   {`{"colour":"blue"}`, CodeBadRequest},
		"unknown field beside a known one": {
			`{"name":"a","colour":"blue"}`, CodeBadRequest,
		},
		// The two bad_request rows above and the two below are the same rule:
		// the body parsed and the target cannot take it. A name it does not
		// declare and a value of the wrong type are the two ways that happens.
		"wrong type":                     {`{"count":"three"}`, CodeBadRequest},
		"wrong type beside a good field": {`{"name":"a","count":[1]}`, CodeBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			_, rec, ok := decodeBody(t, tc.body)
			if ok {
				t.Fatalf("DecodeJSON accepted %s", tc.body)
			}
			problemFrom(t, rec, tc.wantCode)
		})
	}
}

func TestDecodeJSONRejectsAWrongTypeAsBadRequestAndNamesTheField(t *testing.T) {
	// bad_request, per §1B as amended: {"count":"three"} is well-formed JSON
	// whose value this endpoint cannot use, and calling that invalid JSON sends
	// the sender to look at their quoting rather than at their value. The
	// message still names the field, which is the half that makes the 400
	// actionable.
	_, rec, ok := decodeBody(t, `{"count":"three"}`)

	if ok {
		t.Fatal("DecodeJSON accepted a string where an int was required")
	}
	p := problemFrom(t, rec, CodeBadRequest)
	if !strings.Contains(p.Message, "count") {
		t.Errorf("message = %q, want it to name the field", p.Message)
	}
}

func TestDecodeJSONRejectsAnEmptyBody(t *testing.T) {
	_, rec, ok := decodeBody(t, "")

	if ok {
		t.Fatal("DecodeJSON accepted an empty body")
	}
	problemFrom(t, rec, CodeInvalidJSON)
}

func TestDecodeJSONRejectsTrailingContent(t *testing.T) {
	// Without the second Decode this passes silently, having read the first
	// object and dropped the rest.
	_, rec, ok := decodeBody(t, `{"name":"a"}{"name":"b"}`)

	if ok {
		t.Fatal("DecodeJSON accepted two JSON values in one body")
	}
	problemFrom(t, rec, CodeInvalidJSON)
}

func TestDecodeJSONRejectsABodyOverTheLimit(t *testing.T) {
	// One byte of padding past the limit is enough; the reader errors as soon
	// as the limit is crossed, so nothing here allocates a second megabyte.
	_, rec, ok := decodeBody(t, `{"name":"`+strings.Repeat("a", MaxBodyBytes)+`"}`)

	if ok {
		t.Fatal("DecodeJSON accepted a body over MaxBodyBytes")
	}
	p := problemFrom(t, rec, CodeInvalidJSON)
	if !strings.Contains(p.Message, "1048576") {
		t.Errorf("message = %q, want it to state the byte limit", p.Message)
	}
}

func TestDecodeJSONReportsTheLimitWhenTheOverflowTrailsAValidValue(t *testing.T) {
	// An over-limit body whose first value parsed lands in the trailing-content
	// branch rather than the decode branch, and "exactly one JSON object" would
	// name the wrong fault there: the body is not two objects, it is too big.
	_, rec, ok := decodeBody(t, `{"name":"a"}`+strings.Repeat(" ", MaxBodyBytes))

	if ok {
		t.Fatal("DecodeJSON accepted a body over MaxBodyBytes")
	}
	p := problemFrom(t, rec, CodeInvalidJSON)
	if !strings.Contains(p.Message, "1048576") {
		t.Errorf("message = %q, want the byte limit rather than a complaint about a second value", p.Message)
	}
}

func TestDecodeJSONFailureIsItselfAProblemResponse(t *testing.T) {
	// DecodeJSON writes the response itself and returns false; a handler that
	// then writes its own 400 would send two status lines. This pins the
	// headers so the caller can see it has already answered.
	_, rec, _ := decodeBody(t, `{`)

	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	problemFrom(t, rec, CodeInvalidJSON)
}
