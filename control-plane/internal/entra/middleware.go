package entra

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// CodeUnauthenticated is the problem code Middleware writes with its 401.
const CodeUnauthenticated = "unauthenticated"

// codeTenantNotEnrolled is the Hub contract's 403 code (§4.2).
const codeTenantNotEnrolled = "tenant_not_enrolled"

type claimsKey struct{}

// ClaimsFrom returns the Claims Middleware stored.
func ClaimsFrom(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(Claims)
	return c, ok
}

// errNoToken: the request carries no bearer credential at all.
var errNoToken = errors.New("entra: no bearer token")

// reasons maps each rejection to the Hub contract's details.reason (§4.2).
var reasons = []struct {
	err    error
	reason string
}{
	{ErrMalformed, "malformed"},
	{ErrSignature, "signature"},
	{ErrExpired, "expired"},
	{ErrNotYetValid, "not_yet_valid"},
	{ErrIssuer, "issuer"},
	{ErrAudience, "audience"},
	{ErrScope, "scope"},
	{ErrClient, "client"},
	{ErrVersion, "version"},
}

// Middleware authenticates "Authorization: Bearer <token>", stores the Claims
// in the request context, and REMOVES the Authorization header from the
// request it hands on, so no handler can forward the user's token (ADR-H1
// §5.8, 67609ab :543-546: no token is forwarded to another service or into a
// sandbox).
//
// No Bearer credential (no Authorization header, another scheme, or an empty
// token): 401, a bare "WWW-Authenticate: Bearer" (no error code, RFC 6750
// §3.1), details.reason "missing". A rejection: 401, `Bearer
// error="invalid_token"` — `insufficient_scope` for ErrScope (Hub contract
// §4.1) — with details.reason; ErrTenant is 403 tenant_not_enrolled with
// details.tenantId when the validator named a GUID tenant (§4.2).
// ErrKeySetUnavailable: 503 backend_unavailable. An error that is none of
// these: 500 internal. Bodies are httpx problems with code
// CodeUnauthenticated (401), tenant_not_enrolled (403),
// httpx.CodeBackendUnavailable (503) or httpx.CodeInternal (500), and never
// hold the token or the error chain.
func Middleware(v Validator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header)
		if errors.Is(err, errNoToken) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			httpx.WriteProblem(w, http.StatusUnauthorized, CodeUnauthenticated,
				"This request needs a Microsoft Entra access token.", map[string]string{"reason": "missing"})
			return
		}
		if err == nil {
			var claims Claims
			claims, err = v.Validate(r.Context(), token)
			if err == nil {
				r = r.Clone(context.WithValue(r.Context(), claimsKey{}, claims))
				r.Header.Del("Authorization")
				next.ServeHTTP(w, r)
				return
			}
		}
		writeRejection(w, err)
	})
}

// bearerToken reads the one Authorization header. A scheme other than Bearer
// (compared case-insensitively, RFC 7235 §2.1) is no bearer token; two headers
// are a malformed request, because which one a proxy saw is unknowable.
func bearerToken(h http.Header) (string, error) {
	vals := h.Values("Authorization")
	switch {
	case len(vals) == 0:
		return "", errNoToken
	case len(vals) > 1:
		return "", malformed("more than one Authorization header")
	}
	scheme, rest, _ := strings.Cut(strings.TrimSpace(vals[0]), " ")
	token := strings.TrimSpace(rest)
	if !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", errNoToken
	}
	return token, nil
}

func writeRejection(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrKeySetUnavailable) {
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
			"Sign-in cannot be checked right now: Microsoft Entra's signing keys are unreachable.", nil)
		return
	}
	if errors.Is(err, ErrTenant) {
		var details map[string]string
		var te *tenantError
		if errors.As(err, &te) && isGUID(te.tid) {
			details = map[string]string{"tenantId": te.tid}
		}
		httpx.WriteProblem(w, http.StatusForbidden, codeTenantNotEnrolled,
			"Your organisation's Microsoft Entra tenant is not enrolled.", details)
		return
	}
	for _, r := range reasons {
		if errors.Is(err, r.err) {
			challenge := `Bearer error="invalid_token"`
			if r.err == ErrScope {
				challenge = `Bearer error="insufficient_scope"`
			}
			w.Header().Set("WWW-Authenticate", challenge)
			httpx.WriteProblem(w, http.StatusUnauthorized, CodeUnauthenticated,
				"The access token was not accepted.", map[string]string{"reason": r.reason})
			return
		}
	}
	httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeInternal,
		"The access token could not be checked.", nil)
}
