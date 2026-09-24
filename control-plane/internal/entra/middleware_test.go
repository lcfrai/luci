package entra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// fakeValidator answers whatever it is told and records what it was asked.
type fakeValidator struct {
	claims Claims
	err    error
	calls  int
	got    string
}

func (f *fakeValidator) Validate(_ context.Context, bearer string) (Claims, error) {
	f.calls++
	f.got = bearer
	return f.claims, f.err
}

type served struct {
	rec     *httptest.ResponseRecorder
	problem httpx.Problem
	reached bool
}

func serve(t *testing.T, v Validator, auth ...string) served {
	t.Helper()
	var s served
	h := Middleware(v, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.reached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/sandboxes", nil)
	for _, a := range auth {
		req.Header.Add("Authorization", a)
	}
	s.rec = httptest.NewRecorder()
	h.ServeHTTP(s.rec, req)
	if s.rec.Code != http.StatusNoContent {
		if err := json.Unmarshal(s.rec.Body.Bytes(), &s.problem); err != nil {
			t.Fatalf("status %d, body not a problem: %q", s.rec.Code, s.rec.Body.String())
		}
	}
	return s
}

func TestMiddlewareStatuses(t *testing.T) {
	const tok = "eyJhbGciOiJSUzI1NiJ9.eyJ0aWQiOiJ4In0.c2lnbmF0dXJlLXNlZ21lbnQ"
	type want struct {
		status    int
		challenge string // WWW-Authenticate; "" means absent
		code      string
		reason    string
	}
	bearer := `Bearer error="invalid_token"`
	cases := []struct {
		name   string
		auth   []string
		err    error
		want   want
		called bool
	}{
		{"no header", nil, nil, want{401, "Bearer", CodeUnauthenticated, "missing"}, false},
		{"basic", []string{"Basic dXNlcjpwYXNz"}, nil, want{401, "Bearer", CodeUnauthenticated, "missing"}, false},
		{"empty bearer", []string{"Bearer "}, nil, want{401, "Bearer", CodeUnauthenticated, "missing"}, false},
		{"bare scheme", []string{"Bearer"}, nil, want{401, "Bearer", CodeUnauthenticated, "missing"}, false},
		{"two headers", []string{"Bearer " + tok, "Bearer " + tok}, nil, want{401, bearer, CodeUnauthenticated, "malformed"}, false},
		{"malformed", []string{"Bearer " + tok}, ErrMalformed, want{401, bearer, CodeUnauthenticated, "malformed"}, true},
		{"signature", []string{"Bearer " + tok}, ErrSignature, want{401, bearer, CodeUnauthenticated, "signature"}, true},
		{"expired", []string{"Bearer " + tok}, ErrExpired, want{401, bearer, CodeUnauthenticated, "expired"}, true},
		{"not yet valid", []string{"Bearer " + tok}, ErrNotYetValid, want{401, bearer, CodeUnauthenticated, "not_yet_valid"}, true},
		{"issuer", []string{"Bearer " + tok}, ErrIssuer, want{401, bearer, CodeUnauthenticated, "issuer"}, true},
		{"audience", []string{"Bearer " + tok}, ErrAudience, want{401, bearer, CodeUnauthenticated, "audience"}, true},
		{"client", []string{"Bearer " + tok}, ErrClient, want{401, bearer, CodeUnauthenticated, "client"}, true},
		{"version", []string{"Bearer " + tok}, ErrVersion, want{401, bearer, CodeUnauthenticated, "version"}, true},
		// Hub contract §4.1: a missing scope is insufficient_scope, still 401.
		{"scope", []string{"Bearer " + tok}, ErrScope, want{401, `Bearer error="insufficient_scope"`, CodeUnauthenticated, "scope"}, true},
		{"tenant", []string{"Bearer " + tok}, ErrTenant, want{403, "", "tenant_not_enrolled", ""}, true},
		{"outage", []string{"Bearer " + tok}, ErrKeySetUnavailable, want{503, "", httpx.CodeBackendUnavailable, ""}, true},
		{"unknown error", []string{"Bearer " + tok}, errors.New("boom"), want{500, "", httpx.CodeInternal, ""}, true},
	}
	for _, tc := range cases {
		f := &fakeValidator{err: tc.err}
		if tc.err != nil {
			f.err = fmt.Errorf("%w: chain-xyzzy", tc.err) // as the validator wraps them
		}
		s := serve(t, f, tc.auth...)
		if s.reached {
			t.Errorf("%s: the handler ran", tc.name)
		}
		if s.rec.Code != tc.want.status {
			t.Errorf("%s: status %d, want %d", tc.name, s.rec.Code, tc.want.status)
		}
		if got := s.rec.Header().Get("WWW-Authenticate"); got != tc.want.challenge {
			t.Errorf("%s: WWW-Authenticate %q, want %q", tc.name, got, tc.want.challenge)
		}
		if s.problem.Error != tc.want.code || s.problem.Details["reason"] != tc.want.reason {
			t.Errorf("%s: problem %+v, want code %q reason %q", tc.name, s.problem, tc.want.code, tc.want.reason)
		}
		if (f.calls == 1) != tc.called {
			t.Errorf("%s: validator called %d times", tc.name, f.calls)
		}
		if strings.Contains(s.rec.Body.String(), tok) || strings.Contains(s.rec.Body.String(), "xyzzy") {
			t.Errorf("%s: the body holds the token or the error chain: %s", tc.name, s.rec.Body.String())
		}
	}
	// The scheme is case-insensitive (RFC 7235 §2.1); the token is passed as is.
	f := &fakeValidator{}
	if s := serve(t, f, "bearer "+tok); !s.reached || f.got != tok {
		t.Errorf("lower-case scheme: reached=%v, validator got %q", s.reached, f.got)
	}
	// The real validator's tenant refusal names the tenant (contract §4.2).
	is, clk := newIssuer(t), newClock()
	s := serve(t, newValidator(t, is, clk), "Bearer "+mint(t, testKey(t), hdr(kid1), claimsFor(is, tidC, t0)))
	if s.rec.Code != 403 || s.problem.Error != "tenant_not_enrolled" || s.problem.Details["tenantId"] != tidC {
		t.Errorf("real tenant refusal: %d %+v", s.rec.Code, s.problem)
	}
}

func TestMiddlewareStripsTheAuthorizationHeader(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	var (
		sawAuth []string
		claims  Claims
		ok      bool
	)
	h := Middleware(v, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Values("Authorization")
		claims, ok = ClaimsFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", nil)
	req.Header.Set("Authorization", "Bearer "+good(t, is, tidA, t0))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(sawAuth) != 0 {
		t.Errorf("the handler saw Authorization %q", sawAuth)
	}
	if !ok || claims.Key() != tidA+"/"+userOID {
		t.Errorf("ClaimsFrom = %+v, %v", claims, ok)
	}
	if _, ok := ClaimsFrom(context.Background()); ok {
		t.Error("ClaimsFrom on a bare context: ok")
	}
}
