package entra

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// maxTokenBytes bounds what is decoded at all. An Entra access token is a
	// few kilobytes; group claims overflow to a link at 200 groups.
	maxTokenBytes = 32 << 10
	// maxNumericDate is 9999-12-31T23:59:59Z. Past it, time arithmetic is not
	// trusted, and no issuer writes it.
	maxNumericDate = 253402300799
)

// No error in this file quotes the token, a segment of it, or a claim other
// than a GUID tid: rejections are logged, and a token in a log is a credential
// in a log.

type header struct {
	Alg  string          `json:"alg"`
	Kid  string          `json:"kid"`
	Crit json.RawMessage `json:"crit"`
}

type payload struct {
	Ver               string   `json:"ver"`
	Iss               string   `json:"iss"`
	Aud               string   `json:"aud"`
	Tid               string   `json:"tid"`
	Oid               string   `json:"oid"`
	Azp               string   `json:"azp"`
	Scp               string   `json:"scp"`
	Roles             []string `json:"roles"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	Exp               *int64   `json:"exp"`
	Nbf               *int64   `json:"nbf"`
}

type token struct {
	signingInput string // header.payload, as sent
	kid          string
	sig          []byte
	claims       payload
}

func malformed(why string) error { return fmt.Errorf("%w: %s", ErrMalformed, why) }

// tenantError is ErrTenant carrying the token's tid, so the 403 and its log
// line can name the tenant (Hub contract §4.2, §11.5). The tid is unverified —
// the tenant is checked before the signature — so it is only ever quoted when
// it is a GUID.
type tenantError struct{ tid string }

func (e *tenantError) Error() string {
	if isGUID(e.tid) {
		return ErrTenant.Error() + ": " + e.tid
	}
	return ErrTenant.Error() + ": tid absent or not a GUID"
}

func (e *tenantError) Unwrap() error { return ErrTenant }

// Validate checks bearer, the token text without its "Bearer " scheme, in the
// Hub contract's order (§6.3): shape, tenant, signature, ver, iss, aud, exp and
// nbf, azp, scp, oid. The first failure is returned.
func (v *JWKSValidator) Validate(ctx context.Context, bearer string) (Claims, error) {
	tok, err := parseToken(bearer)
	if err != nil {
		return Claims{}, err
	}
	p := tok.claims
	if !v.enrolled[p.Tid] {
		return Claims{}, &tenantError{tid: p.Tid}
	}
	key, err := v.key(ctx, p.Tid, tok.kid)
	if err != nil {
		return Claims{}, err
	}
	sum := sha256.Sum256([]byte(tok.signingInput))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], tok.sig); err != nil {
		return Claims{}, fmt.Errorf("%w: RS256 verification failed", ErrSignature)
	}
	if p.Ver != "2.0" {
		return Claims{}, fmt.Errorf("%w: ver is not 2.0", ErrVersion)
	}
	if p.Iss != v.authority+"/"+p.Tid+"/v2.0" {
		return Claims{}, fmt.Errorf("%w: iss is not this authority's issuer for the token's tenant", ErrIssuer)
	}
	if p.Aud != v.audience {
		return Claims{}, fmt.Errorf("%w: aud is not this API's client id", ErrAudience)
	}
	if p.Exp == nil || p.Nbf == nil {
		return Claims{}, malformed("exp and nbf are both required")
	}
	now := v.now()
	exp, nbf := time.Unix(*p.Exp, 0), time.Unix(*p.Nbf, 0)
	if !now.Before(exp.Add(v.skew)) {
		return Claims{}, fmt.Errorf("%w: exp plus the clock skew has passed", ErrExpired)
	}
	if nbf.Add(-v.skew).After(now) {
		return Claims{}, fmt.Errorf("%w: nbf less the clock skew is still ahead", ErrNotYetValid)
	}
	if !v.clients[p.Azp] {
		return Claims{}, fmt.Errorf("%w: azp is not an allowed client", ErrClient)
	}
	scopes := strings.Fields(p.Scp)
	if !contains(scopes, v.scope) {
		return Claims{}, fmt.Errorf("%w: scp does not hold %s", ErrScope, v.scope)
	}
	if !isGUID(p.Oid) {
		return Claims{}, malformed("oid absent or not a GUID")
	}
	return Claims{
		TenantID:          p.Tid,
		ObjectID:          p.Oid,
		ClientID:          p.Azp,
		Name:              p.Name,
		PreferredUsername: p.PreferredUsername,
		Email:             p.Email,
		Scopes:            scopes,
		Roles:             p.Roles,
		ExpiresAt:         exp,
	}, nil
}

// parseToken is the shape check: three base64url segments, a header that is a
// JSON object with alg exactly RS256, a kid and no crit, then a payload that
// is a JSON object whose claims have their JSON types. The header is judged
// before the payload is decoded, so none, HS* and every other alg are refused
// before a claim is read.
func parseToken(s string) (*token, error) {
	if s == "" || len(s) > maxTokenBytes {
		return nil, malformed("empty, or over 32 KiB")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' {
			return nil, malformed("a character outside unpadded base64url")
		}
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, malformed("not three segments")
	}
	var h header
	if err := decodeObject(parts[0], &h); err != nil {
		return nil, malformed("header: " + err.Error())
	}
	if h.Alg != "RS256" {
		return nil, malformed("alg is not RS256")
	}
	if h.Kid == "" {
		return nil, malformed("no kid")
	}
	if len(h.Crit) > 0 {
		return nil, malformed("crit names extensions this validator does not implement")
	}
	var p payload
	if err := decodeObject(parts[1], &p); err != nil {
		return nil, malformed("payload: " + err.Error())
	}
	for _, d := range []*int64{p.Exp, p.Nbf} {
		if d != nil && (*d < 0 || *d > maxNumericDate) {
			return nil, malformed("exp or nbf out of range")
		}
	}
	sig, err := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if err != nil || len(sig) == 0 {
		return nil, malformed("signature segment empty or not base64url")
	}
	return &token{signingInput: parts[0] + "." + parts[1], kid: h.Kid, sig: sig, claims: p}, nil
}

// decodeObject decodes one base64url segment holding a JSON object into dst.
// Its errors are fixed strings: a json error can quote the input.
func decodeObject(seg string, dst any) error {
	b, err := base64.RawURLEncoding.Strict().DecodeString(seg)
	if err != nil {
		return errBase64
	}
	if t := bytes.TrimLeft(b, " \t\r\n"); len(t) == 0 || t[0] != '{' {
		return errNotObject
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return errClaimType
	}
	return nil
}

var (
	errBase64    = errors.New("not unpadded base64url")
	errNotObject = errors.New("not a JSON object")
	errClaimType = errors.New("not JSON, or a member of the wrong type")
)

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
