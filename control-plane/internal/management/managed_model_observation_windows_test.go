//go:build windows

package management

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"golang.org/x/sys/windows"
)

// These tests use a controlled RoundTripper, not a listener or provider.
type managedObservationRoundTrip func(*http.Request) (*http.Response, error)

func (f managedObservationRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type managedObservationBody struct {
	io.Reader
	closeErr error
	closed   int
}

func (b *managedObservationBody) Close() error { b.closed++; return b.closeErr }

type managedObservationReadError struct{}

func (managedObservationReadError) Read([]byte) (int, error) {
	return 0, errors.New("private body read failure")
}

type managedObservationCarrier struct {
	detail *managedmodel.ObservationDiagnostic
}

func (e managedObservationCarrier) Error() string { return "private observation context" }
func (e managedObservationCarrier) ManagedModelObservationDiagnostic() *managedmodel.ObservationDiagnostic {
	return e.detail
}

func TestManagedMetadataClassifiesWrappedTransportWithoutRetryingSemanticFailures(t *testing.T) {
	wrap := func(e error) error { return &url.Error{Op: "Get", URL: "http://private-endpoint/private", Err: e} }
	for _, test := range []struct {
		name, stage, reason string
		err                 error
	}{
		{"plain", "show", "request_failed", errors.New("private semantic failure")},
		{"wrapped_plain", "show", "request_failed", wrap(errors.New("private semantic failure"))},
		{"wrapped_peer", "peer", "ownership_unavailable", wrap(managedObservationFailure("peer", "ownership_unavailable", errors.New("private peer"), 0))},
		{"wrapped_listener", "listener", "ownership_unavailable", wrap(managedObservationFailure("listener", "ownership_unavailable", context.DeadlineExceeded, 0))},
		{"deadline", "show", "timeout", wrap(context.DeadlineExceeded)},
		{"cancelled", "show", "cancelled", wrap(context.Canceled)},
		{"timeout", "show", "timeout", wrap(&net.DNSError{Err: "private DNS", IsTimeout: true})},
		{"refused", "show", "connect_refused", wrap(windows.WSAECONNREFUSED)},
		{"reset", "show", "connection_lost", wrap(windows.WSAECONNRESET)},
		{"aborted", "show", "connection_lost", wrap(windows.WSAECONNABORTED)},
		{"eof", "show", "connection_lost", wrap(io.EOF)},
		{"short_body", "show", "connection_lost", wrap(io.ErrUnexpectedEOF)},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := managedMetadataRequestFailure("show", "request_failed", test.err, 0)
			d := managedObservationDiagnostic(err)
			if d == nil || d.Stage != test.stage || d.Reason != test.reason || d.HTTPStatus != nil {
				t.Fatalf("wrong sanitized classification: %+v", d)
			}
			raw, e := json.Marshal(d)
			if e != nil || strings.Contains(string(raw), "private") {
				t.Fatal("raw transport context exposed")
			}
		})
	}
}

func TestManagedMetadataRequestPreservesRouteStatusAndBoundedResponse(t *testing.T) {
	for _, test := range []struct {
		name, path, body, raw, stage, reason string
		status                               int
		readErr, closeErr, verifyErr         bool
		cancelVerify                         bool
	}{
		{name: "tags", path: "/api/tags", raw: `{"models":[]}`, status: 200},
		{name: "show", path: "/api/show", body: `{"model":"private-model"}`, raw: `{"parameters":"num_ctx 16384"}`, status: 200},
		{name: "ps", path: "/api/ps", raw: `{"models":[]}`, status: 200},
		{name: "http_error", path: "/api/show", raw: "private vendor response", status: 503, stage: "show", reason: "http_status"},
		{name: "redirect", path: "/api/tags", raw: "private redirect", status: 302, stage: "tags", reason: "http_status"},
		{name: "invalid_json", path: "/api/show", raw: "private invalid JSON", status: 200, stage: "show", reason: "response_invalid"},
		{name: "oversize", path: "/api/ps", raw: strings.Repeat("x", (256<<10)+1), status: 200, stage: "ps", reason: "response_too_large"},
		{name: "read_error", path: "/api/tags", status: 200, readErr: true, stage: "tags", reason: "response_read_failed"},
		{name: "close_error", path: "/api/ps", raw: `{"models":[]}`, status: 200, closeErr: true, stage: "ps", reason: "response_close_failed"},
		{name: "listener_changed", path: "/api/show", raw: `{}`, status: 200, verifyErr: true, stage: "listener", reason: "ownership_unavailable"},
		{name: "cancel_after_response", path: "/api/show", raw: `{}`, status: 200, cancelVerify: true, stage: "show", reason: "cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			body := &managedObservationBody{Reader: strings.NewReader(test.raw)}
			if test.readErr {
				body.Reader = managedObservationReadError{}
			}
			if test.closeErr {
				body.closeErr = errors.New("private close error")
			}
			calls, verified := 0, 0
			client := &http.Client{Transport: managedObservationRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				method := http.MethodGet
				if test.body != "" {
					method = http.MethodPost
				}
				if r.Context() != ctx || r.URL.String() != "http://127.0.0.1:24567"+test.path || r.Method != method || r.Header.Get("Content-Type") != "application/json" {
					t.Fatal("selected route, method or original context changed")
				}
				if r.Body != nil {
					raw, e := io.ReadAll(r.Body)
					if e != nil || string(raw) != test.body {
						t.Fatal("selected model request changed")
					}
				}
				return &http.Response{StatusCode: test.status, Header: make(http.Header), Body: body, Request: r}, nil
			}), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			var out map[string]any
			err := managedModelMetadata(ctx, client, "http://127.0.0.1:24567", test.path, test.body, &out, func() error {
				verified++
				if test.cancelVerify {
					cancel()
				}
				if test.verifyErr {
					return errors.New("private listener changed")
				}
				return nil
			})
			if calls != 1 || verified != 1 || body.closed != 1 {
				t.Fatalf("unexpected retry, missing verification or unchecked response: %d/%d/%d", calls, verified, body.closed)
			}
			if test.reason == "" {
				if err != nil || len(out) == 0 {
					t.Fatal("valid selected metadata refused", err)
				}
				return
			}
			d := managedObservationDiagnostic(err)
			if d == nil || d.Stage != test.stage || d.Reason != test.reason || d.HTTPStatus == nil || *d.HTTPStatus != test.status {
				t.Fatalf("wrong fixed response diagnostic: %+v", d)
			}
			raw, e := json.Marshal(d)
			if e != nil || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "127.0.0.1") {
				t.Fatal("private response context exposed")
			}
		})
	}
}

func TestManagedMetadataStartupUsesOriginalDeadlineAndStopsRetryAfterReadiness(t *testing.T) {
	for _, test := range []struct {
		name, reason, want string
		before, ready      bool
		wait               bool
	}{
		{"timeout_before", "timeout", "starting", true, false, true},
		{"refused_before", "connect_refused", "starting", true, false, true},
		{"lost_before", "connection_lost", "starting", true, false, true},
		{"timeout_expired", "timeout", "startup_timeout", false, false, false},
		{"refused_expired", "connect_refused", "startup_timeout", false, false, false},
		{"lost_expired", "connection_lost", "startup_timeout", false, false, false},
		{"semantic_before", "ownership_unavailable", "listener_ownership_unavailable", true, false, false},
		{"http_before", "http_status", "", true, false, false},
		{"json_before", "response_invalid", "", true, false, false},
		{"cancel_before", "cancelled", "", true, false, false},
		{"unknown_before", "request_failed", "", true, false, false},
		{"ready_timeout_before", "timeout", "", true, true, false},
		{"ready_timeout_after", "timeout", "", false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			deadline := time.Now().Add(-time.Minute)
			if test.before {
				deadline = time.Now().Add(time.Minute)
			}
			s := windowsManagedModelService{startupDeadline: deadline, metadataReady: test.ready}
			err := managedObservationFailure("show", test.reason, errors.New("private cause"), 0)
			result, got := s.metadataFailure(managedModelSample{}, err)
			if result.failure != test.want || (got == nil) != test.wait || !s.startupDeadline.Equal(deadline) || s.metadataReady != test.ready {
				t.Fatal("classification changed deadline/readiness or delayed a semantic failure")
			}
			if !test.wait && (got != err || managedObservationDiagnostic(got).Reason != test.reason) {
				t.Fatal("original sanitized failure lost")
			}
		})
	}
}

func TestManagedObservationProjectionDropsUnknownValuesAndCopiesStatus(t *testing.T) {
	status := 503
	source := &managedmodel.ObservationDiagnostic{Stage: "show", Reason: "http_status", HTTPStatus: &status}
	err := errors.Join(errors.New("private wrapper"), managedObservationCarrier{source})
	copy := managedObservationDiagnostic(err)
	status = 200
	source.Stage = "private-stage"
	if copy == nil || copy.Stage != "show" || copy.HTTPStatus == nil || *copy.HTTPStatus != 503 {
		t.Fatal("observation projection aliases private source")
	}
	if managedObservationDiagnostic(err) != nil {
		t.Fatal("unknown stage exposed")
	}
	source.Stage, source.Reason = "show", "private-reason"
	if managedObservationDiagnostic(err) != nil {
		t.Fatal("unknown reason exposed")
	}
	source.Reason = "http_status"
	for _, invalid := range []int{0, 99, 600} {
		status = invalid
		if projected := managedObservationDiagnostic(err); projected == nil || projected.HTTPStatus != nil {
			t.Fatal("invalid numeric HTTP status exposed")
		}
	}
	if managedObservationDiagnostic(errors.New("private untyped error")) != nil {
		t.Fatal("untyped raw cause projected")
	}
}
