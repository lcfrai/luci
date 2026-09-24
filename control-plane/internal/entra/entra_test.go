package entra

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

var bg = context.Background()

func TestAValidTokenYieldsItsClaims(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	c, err := v.Validate(bg, good(t, is, tidA, clk.Now()))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := Claims{
		TenantID:          tidA,
		ObjectID:          userOID,
		ClientID:          desktop,
		Name:              "Ada Lovelace",
		PreferredUsername: "ada@example.test",
		Email:             "ada.lovelace@example.test",
		Scopes:            []string{testScope},
		Roles:             []string{"Reader"},
	}
	if !c.ExpiresAt.Equal(t0.Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v, want %v", c.ExpiresAt, t0.Add(time.Hour))
	}
	c.ExpiresAt = time.Time{}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("claims = %+v\nwant     %+v", c, want)
	}
	if got := c.Key(); got != tidA+"/"+userOID {
		t.Errorf("Key() = %q, want tid/oid", got)
	}
	// The second enrolled tenant, from its own key set.
	c, err = v.Validate(bg, good(t, is, tidB, clk.Now()))
	if err != nil || c.TenantID != tidB {
		t.Fatalf("tenant B: %+v, %v", c, err)
	}
	if is.fetches(tidA) != 1 || is.fetches(tidB) != 1 {
		t.Errorf("fetches A=%d B=%d, want 1 each", is.fetches(tidA), is.fetches(tidB))
	}
}

func TestNoneIsMalformed(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	claims := claimsFor(is, tidA, clk.Now())
	valid := good(t, is, tidA, clk.Now())
	realSig := valid[strings.LastIndex(valid, ".")+1:]
	for name, tok := range map[string]string{
		"none, empty signature":      seg(t, map[string]any{"alg": "none", "typ": "JWT"}) + "." + seg(t, claims) + ".",
		"none with a kid":            seg(t, map[string]any{"alg": "none", "kid": kid1}) + "." + seg(t, claims) + ".",
		"None with a real sig":       seg(t, map[string]any{"alg": "None", "kid": kid1}) + "." + seg(t, claims) + "." + realSig,
		"no alg at all":              seg(t, map[string]any{"kid": kid1}) + "." + seg(t, claims) + "." + realSig,
		"RS256 with no signature":    seg(t, hdr(kid1)) + "." + seg(t, claims) + ".",
		"RS256 as two segments only": seg(t, hdr(kid1)) + "." + seg(t, claims),
	} {
		_, err := v.Validate(bg, tok)
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		isOnly(t, err, ErrMalformed)
	}
	if n := is.total(); n != 0 {
		t.Errorf("%d key-set fetches, want 0: alg is refused before the key set is read", n)
	}
}

func TestHS256SignedWithThePublicKeyIsMalformed(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	claims := claimsFor(is, tidA, clk.Now())
	// The key-confusion attack: an HMAC keyed with the public modulus, which
	// anyone can read from the JWKS.
	secret := testKey(t).PublicKey.N.Bytes()
	for _, alg := range []string{"HS256", "HS384", "HS512", "hs256", "RS384", "PS256", "ES256", "rs256", "RS256 "} {
		input := seg(t, map[string]any{"alg": alg, "typ": "JWT", "kid": kid1}) + "." + seg(t, claims)
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(input))
		_, err := v.Validate(bg, input+"."+b64(mac.Sum(nil)))
		if err == nil {
			t.Fatalf("alg %q: accepted", alg)
		}
		isOnly(t, err, ErrMalformed)
	}
	if n := is.total(); n != 0 {
		t.Errorf("%d key-set fetches, want 0", n)
	}
}

func TestAMissingKidIsMalformed(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	claims := claimsFor(is, tidA, clk.Now())
	for name, h := range map[string]map[string]any{
		"absent":    {"alg": "RS256", "typ": "JWT"},
		"empty":     {"alg": "RS256", "typ": "JWT", "kid": ""},
		"not a str": {"alg": "RS256", "typ": "JWT", "kid": 7},
	} {
		_, err := v.Validate(bg, mint(t, testKey(t), h, claims))
		if err == nil {
			t.Fatalf("kid %s: accepted", name)
		}
		isOnly(t, err, ErrMalformed)
	}
	if n := is.total(); n != 0 {
		t.Errorf("%d key-set fetches, want 0: no kid, no key to try", n)
	}
}

func TestATamperedPayloadFailsTheSignature(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	now := clk.Now()
	valid := good(t, is, tidA, now)
	sig := valid[strings.LastIndex(valid, ".")+1:]
	sigBytes, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		t.Fatal(err)
	}
	claims := claimsFor(is, tidA, now)
	cases := map[string]string{
		// One byte of one claim changed; the old signature kept.
		"payload": seg(t, hdr(kid1)) + "." + seg(t, edit(claims, map[string]any{"name": "Ada Lovelacf"})) + "." + sig,
		// A header the signature does not cover: a verifier over the payload only.
		"header": seg(t, map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid1, "x": 1}) + "." + seg(t, claims) + "." + sig,
		// Signed by a key that is not the published one, under its kid.
		"foreign key": mint(t, newKey(t, 2048), hdr(kid1), claims),
		// The signature one byte short, still canonical base64url.
		"truncated": valid[:strings.LastIndex(valid, ".")+1] + b64(sigBytes[:len(sigBytes)-1]),
		// The check order: a bad signature is reported before the claims are
		// believed, even when the claims are also wrong.
		"and expired": seg(t, hdr(kid1)) + "." + seg(t, edit(claims, map[string]any{"exp": now.Add(-time.Hour).Unix()})) + "." + sig,
	}
	for name, tok := range cases {
		_, err := v.Validate(bg, tok)
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		isOnly(t, err, ErrSignature)
	}
}

func TestTheTenantIsCheckedBeforeAnyFetch(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	now := clk.Now()
	// tidC has a key set the issuer would serve; the validator must not ask.
	for name, claims := range map[string]map[string]any{
		"not enrolled": claimsFor(is, tidC, now),
		"no tid":       edit(claimsFor(is, tidA, now), map[string]any{"tid": drop}),
		"upper case":   edit(claimsFor(is, tidA, now), map[string]any{"tid": strings.ToUpper(tidA)}),
		"path escape":  edit(claimsFor(is, tidA, now), map[string]any{"tid": tidA + "/../" + tidC}),
	} {
		_, err := v.Validate(bg, mint(t, testKey(t), hdr(kid1), claims))
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		isOnly(t, err, ErrTenant)
	}
	if n := is.total(); n != 0 {
		t.Errorf("%d key-set fetches, want 0: a caller must not choose whose keys we fetch", n)
	}
}

func TestExpiryAndNotBeforeHonourTheSkew(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	now := clk.Now()
	sec := func(n int) int64 { return now.Add(time.Duration(n) * time.Second).Unix() }
	cases := []struct {
		name    string
		changes map[string]any
		want    error // nil: accepted
	}{
		{"exp 119 s ago", map[string]any{"exp": sec(-119)}, nil},
		{"exp 120 s ago", map[string]any{"exp": sec(-120)}, ErrExpired},
		{"exp 121 s ago", map[string]any{"exp": sec(-121)}, ErrExpired},
		{"nbf in 119 s", map[string]any{"nbf": sec(119)}, nil},
		{"nbf in 120 s", map[string]any{"nbf": sec(120)}, nil},
		{"nbf in 121 s", map[string]any{"nbf": sec(121)}, ErrNotYetValid},
		{"no exp", map[string]any{"exp": drop}, ErrMalformed},
		{"no nbf", map[string]any{"nbf": drop}, ErrMalformed},
		// The check order: expiry is reported before the client.
		{"expired and foreign", map[string]any{"exp": sec(-121), "azp": otherApp}, ErrExpired},
	}
	v := newValidator(t, is, clk)
	for _, tc := range cases {
		_, err := v.Validate(bg, mint(t, testKey(t), hdr(kid1), edit(claimsFor(is, tidA, now), tc.changes)))
		if tc.want == nil {
			if err != nil {
				t.Errorf("%s: %v, want accepted", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: accepted, want %v", tc.name, tc.want)
		}
		isOnly(t, err, tc.want)
	}
	// A configured skew replaces the default on both bounds.
	cfg := testConfig(is, clk)
	cfg.ClockSkew = 10 * time.Second
	v10, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		changes map[string]any
		want    error
	}{
		{map[string]any{"exp": sec(-9)}, nil},
		{map[string]any{"exp": sec(-11)}, ErrExpired},
		{map[string]any{"nbf": sec(9)}, nil},
		{map[string]any{"nbf": sec(11)}, ErrNotYetValid},
	} {
		_, err := v10.Validate(bg, mint(t, testKey(t), hdr(kid1), edit(claimsFor(is, tidA, now), tc.changes)))
		if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("skew 10 s, %v: err = %v, want %v", tc.changes, err, tc.want)
		}
	}
}

// rejects validates tid's valid claims with changes applied and wants exactly want.
func rejects(t *testing.T, v *JWKSValidator, is *issuer, tid string, changes map[string]any, want error) {
	t.Helper()
	_, err := v.Validate(bg, mint(t, testKey(t), hdr(kid1), edit(claimsFor(is, tid, t0), changes)))
	if err == nil {
		t.Fatalf("%v: accepted, want %v", changes, want)
	}
	isOnly(t, err, want)
}

func TestAVersion1TokenIsRejected(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	rejects(t, v, is, tidA, map[string]any{"ver": "1.0"}, ErrVersion)
	rejects(t, v, is, tidA, map[string]any{"ver": drop}, ErrVersion)
	// A real v1 token: sts.windows.net issuer, api:// audience, appid not azp.
	// ver is checked first, so it is reported as the version, not the issuer.
	rejects(t, v, is, tidA, map[string]any{
		"ver": "1.0", "iss": "https://sts.windows.net/" + tidA + "/",
		"aud": "api://" + brokerAPI, "azp": drop, "appid": desktop,
	}, ErrVersion)
}

func TestTheIssuerMustBeTheTokensOwnTenant(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	// Both tenants are enrolled: B's issuer on A's token is a cross-tenant replay.
	rejects(t, v, is, tidA, map[string]any{"iss": is.iss(tidB)}, ErrIssuer)
	rejects(t, v, is, tidA, map[string]any{"iss": is.iss(tidA) + "/"}, ErrIssuer)
	rejects(t, v, is, tidA, map[string]any{"iss": "https://login.microsoftonline.com/" + tidA + "/v2.0"}, ErrIssuer)
	rejects(t, v, is, tidA, map[string]any{"iss": drop}, ErrIssuer)
	// The check order: the issuer before the audience.
	rejects(t, v, is, tidA, map[string]any{"iss": is.iss(tidB), "aud": hubAPI}, ErrIssuer)
}

func TestTheAudienceIsTheClientIDOnly(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	rejects(t, v, is, tidA, map[string]any{"aud": "api://" + brokerAPI}, ErrAudience)
	rejects(t, v, is, tidA, map[string]any{"aud": drop}, ErrAudience)
	// A Hub token at the broker (P8-D4): signed, same tenant, same user.
	hubToken := map[string]any{"aud": hubAPI, "scp": "Hub.Access"}
	rejects(t, v, is, tidA, hubToken, ErrAudience)
	// The Hub's own validator takes it, and refuses the broker's.
	cfg := testConfig(is, clk)
	cfg.Audience, cfg.RequiredScope = hubAPI, "Hub.Access"
	hub, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.Validate(bg, mint(t, testKey(t), hdr(kid1), edit(claimsFor(is, tidA, t0), hubToken))); err != nil {
		t.Fatalf("the Hub refused its own token: %v", err)
	}
	rejects(t, hub, is, tidA, nil, ErrAudience)
	// The check order: the audience before the time.
	rejects(t, v, is, tidA, map[string]any{"aud": hubAPI, "exp": t0.Add(-time.Hour).Unix()}, ErrAudience)
}

func TestAnUnlistedClientIsRejected(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	rejects(t, v, is, tidA, map[string]any{"azp": otherApp}, ErrClient)
	rejects(t, v, is, tidA, map[string]any{"azp": drop}, ErrClient)
	// The check order: the client before the scope.
	rejects(t, v, is, tidA, map[string]any{"azp": otherApp, "scp": drop}, ErrClient)
	// A second allowed client is honoured.
	cfg := testConfig(is, clk)
	cfg.AllowedClients = []string{desktop, otherApp}
	two, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := two.Validate(bg, mint(t, testKey(t), hdr(kid1), edit(claimsFor(is, tidA, t0), map[string]any{"azp": otherApp}))); err != nil {
		t.Fatalf("second allowed client refused: %v", err)
	}
}

func TestScopeMatchIsExact(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	for _, scp := range []string{"Sandboxes.AccessAll", "sandboxes.access", "Sandboxes", "Hub.Access", "openid profile", "xSandboxes.Access"} {
		rejects(t, v, is, tidA, map[string]any{"scp": scp}, ErrScope)
	}
	c, err := v.Validate(bg, mint(t, testKey(t), hdr(kid1), edit(claimsFor(is, tidA, t0), map[string]any{"scp": "openid  Sandboxes.Access offline_access"})))
	if err != nil {
		t.Fatalf("a token holding the scope among others: %v", err)
	}
	if want := []string{"openid", testScope, "offline_access"}; !reflect.DeepEqual(c.Scopes, want) {
		t.Errorf("Scopes = %q, want %q", c.Scopes, want)
	}
	// The check order: the scope before the oid.
	rejects(t, v, is, tidA, map[string]any{"scp": "openid", "oid": drop}, ErrScope)
}

func TestAppOnlyTokensFailTheScopeCheck(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	// A client-credentials token: roles, no scp, and Entra's idtyp marker.
	rejects(t, v, is, tidA, map[string]any{"scp": drop, "roles": []string{testScope}, "idtyp": "app"}, ErrScope)
	rejects(t, v, is, tidA, map[string]any{"scp": ""}, ErrScope)
}

func TestAMissingOrNonGUIDOidIsMalformed(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	for _, oid := range []any{drop, "", "not-a-guid", strings.ToUpper(userOID), "{" + userOID + "}", userOID + "0", strings.ReplaceAll(userOID, "-", "")} {
		rejects(t, v, is, tidA, map[string]any{"oid": oid}, ErrMalformed)
	}
}

func TestSubIsNeverTheIdentity(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	key := func(changes map[string]any) string {
		t.Helper()
		c, err := v.Validate(bg, mint(t, testKey(t), hdr(kid1), edit(claimsFor(is, tidA, t0), changes)))
		if err != nil {
			t.Fatal(err)
		}
		return c.Key()
	}
	if a, b := key(map[string]any{"sub": "sub-seen-by-the-broker"}), key(map[string]any{"sub": "sub-seen-by-the-hub"}); a != b {
		t.Errorf("same tid/oid, different sub: keys %q and %q differ", a, b)
	}
	other := "8de4921a-d1af-4e02-9af9-bfc1d2e3f405"
	if a, b := key(map[string]any{"sub": "same"}), key(map[string]any{"sub": "same", "oid": other}); a == b {
		t.Errorf("different oid, same sub: keys are both %q", a)
	}
	if a, b := key(map[string]any{"preferred_username": "old@example.test"}), key(map[string]any{"preferred_username": "new@example.test"}); a != b {
		t.Errorf("a renamed user changed key: %q and %q", a, b)
	}
}

func TestErrorsNeverContainTheToken(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	is.fail(tidB, 500)
	k := testKey(t)
	c := claimsFor(is, tidA, t0)
	with := func(changes map[string]any) string { return mint(t, k, hdr(kid1), edit(c, changes)) }
	valid := with(nil)
	sig := valid[strings.LastIndex(valid, ".")+1:]
	cases := []struct {
		name string
		tok  string
		want error
	}{
		{"one segment", "eyJhbGciOiJSUzI1NiJ9", ErrMalformed},
		{"four segments", valid + "." + sig, ErrMalformed},
		{"padded base64", strings.Replace(valid, ".", "==.", 1), ErrMalformed},
		{"standard base64", seg(t, hdr(kid1)) + "." + seg(t, c) + "." + strings.NewReplacer("-", "+", "_", "/").Replace(sig) + "+/", ErrMalformed},
		{"header not JSON", b64([]byte("{alg:RS256")) + "." + seg(t, c) + "." + sig, ErrMalformed},
		{"payload null", seg(t, hdr(kid1)) + "." + b64([]byte("null")) + "." + sig, ErrMalformed},
		{"payload array", seg(t, hdr(kid1)) + "." + b64([]byte("[1,2]")) + "." + sig, ErrMalformed},
		{"crit header", mint(t, k, map[string]any{"alg": "RS256", "kid": kid1, "crit": []string{"exp"}}, c), ErrMalformed},
		{"aud array", with(map[string]any{"aud": []string{brokerAPI}}), ErrMalformed},
		{"exp a string", with(map[string]any{"exp": "tomorrow"}), ErrMalformed},
		{"exp past year 9999", with(map[string]any{"exp": int64(1) << 60}), ErrMalformed},
		{"exp before 1970", with(map[string]any{"exp": -1}), ErrMalformed},
		// base64 skips CR and LF itself; the byte scan must not.
		{"a newline inside a segment", valid[:20] + "\n" + valid[20:], ErrMalformed},
		{"oversized", with(map[string]any{"name": strings.Repeat("a", 33<<10)}), ErrMalformed},
		{"alg none", seg(t, map[string]any{"alg": "none", "kid": kid1}) + "." + seg(t, c) + "." + sig, ErrMalformed},
		{"no kid", mint(t, k, map[string]any{"alg": "RS256"}, c), ErrMalformed},
		{"tenant", mint(t, k, hdr(kid1), claimsFor(is, tidC, t0)), ErrTenant},
		{"signature", seg(t, hdr(kid1)) + "." + seg(t, edit(c, map[string]any{"oid": otherApp})) + "." + sig, ErrSignature},
		{"version", with(map[string]any{"ver": "1.0"}), ErrVersion},
		{"issuer", with(map[string]any{"iss": is.iss(tidB)}), ErrIssuer},
		{"audience", with(map[string]any{"aud": hubAPI}), ErrAudience},
		{"expired", with(map[string]any{"exp": t0.Add(-time.Hour).Unix()}), ErrExpired},
		{"not yet valid", with(map[string]any{"nbf": t0.Add(time.Hour).Unix()}), ErrNotYetValid},
		{"client", with(map[string]any{"azp": otherApp}), ErrClient},
		{"scope", with(map[string]any{"scp": "openid"}), ErrScope},
		{"oid", with(map[string]any{"oid": "someone"}), ErrMalformed},
		{"outage", mint(t, k, hdr(kid1), claimsFor(is, tidB, t0)), ErrKeySetUnavailable},
	}
	for _, tc := range cases {
		_, err := v.Validate(bg, tc.tok)
		if err == nil {
			t.Fatalf("%s: accepted", tc.name)
		}
		isOnly(t, err, tc.want)
		msg := err.Error()
		if strings.Contains(msg, tc.tok) {
			t.Errorf("%s: the error holds the token: %s", tc.name, msg)
		}
		for i, part := range strings.Split(tc.tok, ".") {
			if len(part) >= 8 && strings.Contains(msg, part) {
				t.Errorf("%s: the error holds segment %d: %s", tc.name, i, msg)
			}
		}
	}
}

func TestAnInsecureAuthorityIsRefused(t *testing.T) {
	cfg := func(authority string, insecure bool) Config {
		return Config{
			AuthorityHost: authority, AllowInsecureAuthority: insecure,
			TenantIDs: []string{tidA}, Audience: brokerAPI,
			RequiredScope: testScope, AllowedClients: []string{desktop},
		}
	}
	for _, tc := range []struct {
		authority string
		insecure  bool
		ok        bool
	}{
		{"https://login.microsoftonline.com", false, true},
		{"https://login.microsoftonline.com", true, true},
		{"http://127.0.0.1:8740", false, false},
		{"http://127.0.0.1:8740", true, true},
		{"http://[::1]:8740", true, true},
		{"http://example.com", true, false},
		{"http://10.0.0.1:8740", true, false},
		{"http://localhost:8740", true, false},
		{"http://127.0.0.1.nip.io", true, false},
		{"ftp://127.0.0.1", true, false},
		{"login.microsoftonline.com", false, false},
	} {
		_, err := New(cfg(tc.authority, tc.insecure))
		if (err == nil) != tc.ok {
			t.Errorf("New(%q, insecure=%v): err = %v, want ok=%v", tc.authority, tc.insecure, err, tc.ok)
		}
	}
}

// Added by task 16's build: the part's table had no test for New's other
// refusals, so a mutant deleting any of them survived.
func TestAnUnusableConfigIsRefused(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	for name, change := range map[string]func(*Config){
		"no authority":       func(c *Config) { c.AuthorityHost = "" },
		"authority path":     func(c *Config) { c.AuthorityHost += "/common" },
		"authority query":    func(c *Config) { c.AuthorityHost += "/?x=1" },
		"authority fragment": func(c *Config) { c.AuthorityHost += "/#f" },
		"authority userinfo": func(c *Config) { c.AuthorityHost = strings.Replace(c.AuthorityHost, "//", "//u:p@", 1) },
		"no tenants":         func(c *Config) { c.TenantIDs = nil },
		"a domain for a tid": func(c *Config) { c.TenantIDs = []string{"contoso.onmicrosoft.com"} },
		"an empty tid":       func(c *Config) { c.TenantIDs = []string{tidA, ""} },
		"no audience":        func(c *Config) { c.Audience = "" },
		"an api:// audience": func(c *Config) { c.Audience = "api://" + brokerAPI },
		"no scope":           func(c *Config) { c.RequiredScope = "" },
		"two scopes":         func(c *Config) { c.RequiredScope = testScope + " openid" },
		"no clients":         func(c *Config) { c.AllowedClients = nil },
		"a non-GUID client":  func(c *Config) { c.AllowedClients = []string{desktop, "desktop"} },
		"a negative skew":    func(c *Config) { c.ClockSkew = -time.Second },
	} {
		cfg := testConfig(is, clk)
		change(&cfg)
		if _, err := New(cfg); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}
	// Usable variants: a trailing slash and upper-case GUIDs are normalised,
	// so a real token still validates.
	cfg := testConfig(is, clk)
	cfg.AuthorityHost += "/"
	cfg.TenantIDs = []string{strings.ToUpper(tidA)}
	cfg.Audience = strings.ToUpper(brokerAPI)
	cfg.AllowedClients = []string{" " + strings.ToUpper(desktop)}
	v, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if n := is.total(); n != 0 {
		t.Errorf("New made %d fetches, want 0", n)
	}
	if _, err := v.Validate(bg, good(t, is, tidA, t0)); err != nil {
		t.Errorf("normalised config refused a valid token: %v", err)
	}
	// The defaults: a 10 s client, the wall clock.
	cfg = testConfig(is, clk)
	cfg.HTTPClient, cfg.Now = nil, nil
	v, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if v.client == nil || v.client.Timeout != 10*time.Second {
		t.Errorf("default HTTP client = %+v, want a 10 s timeout", v.client)
	}
	if _, err := v.Validate(bg, good(t, is, tidA, time.Now())); err != nil {
		t.Errorf("default clock refused a token minted now: %v", err)
	}
}
