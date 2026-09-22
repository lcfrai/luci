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
// it wrote was not one.
func problemFrom(t *testing.T, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if p.Error != CodeInvalidJSON {
		t.Errorf("error = %q, want %q", p.Error, CodeInvalidJSON)
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

func TestDecodeJSONRejectsAnUnknownField(t *testing.T) {
	// The reason this is a 400 and not a shrug: "worktreedId" silently ignored
	// is a session created against the wrong worktree, which the user then has
	// to notice themselves.
	_, rec, ok := decodeBody(t, `{"name":"api","colour":"blue"}`)

	if ok {
		t.Fatal("DecodeJSON accepted an unknown field")
	}
	p := problemFrom(t, rec)
	if !strings.Contains(p.Message, "colour") {
		t.Errorf("message = %q, want it to name the offending field", p.Message)
	}
}

func TestDecodeJSONNamesTheFieldWithTheWrongType(t *testing.T) {
	_, rec, ok := decodeBody(t, `{"count":"three"}`)

	if ok {
		t.Fatal("DecodeJSON accepted a string where an int was required")
	}
	p := problemFrom(t, rec)
	if !strings.Contains(p.Message, "count") {
		t.Errorf("message = %q, want it to name the field", p.Message)
	}
}

func TestDecodeJSONRejectsAnEmptyBody(t *testing.T) {
	_, rec, ok := decodeBody(t, "")

	if ok {
		t.Fatal("DecodeJSON accepted an empty body")
	}
	problemFrom(t, rec)
}

func TestDecodeJSONRejectsTrailingContent(t *testing.T) {
	// Without the second Decode this passes silently, having read the first
	// object and dropped the rest.
	_, rec, ok := decodeBody(t, `{"name":"a"}{"name":"b"}`)

	if ok {
		t.Fatal("DecodeJSON accepted two JSON values in one body")
	}
	problemFrom(t, rec)
}

func TestDecodeJSONRejectsABodyOverTheLimit(t *testing.T) {
	// One byte of padding past the limit is enough; the reader errors as soon
	// as the limit is crossed, so nothing here allocates a second megabyte.
	_, rec, ok := decodeBody(t, `{"name":"`+strings.Repeat("a", MaxBodyBytes)+`"}`)

	if ok {
		t.Fatal("DecodeJSON accepted a body over MaxBodyBytes")
	}
	p := problemFrom(t, rec)
	if !strings.Contains(p.Message, "1048576") {
		t.Errorf("message = %q, want it to state the byte limit", p.Message)
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
	problemFrom(t, rec)
}
