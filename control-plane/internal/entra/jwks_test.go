package entra

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustValidate(t *testing.T, v *JWKSValidator, tok string) Claims {
	t.Helper()
	c, err := v.Validate(bg, tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return c
}

func wantFetches(t *testing.T, is *issuer, tid string, n int, when string) {
	t.Helper()
	if got := is.fetches(tid); got != n {
		t.Fatalf("%s: %d fetches of %s's key set, want %d", when, got, tid[:8], n)
	}
}

func TestAnUnknownKidRefetchesOnceThenFails(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	unknown := func(tid, kid string) error {
		_, err := v.Validate(bg, mint(t, testKey(t), hdr(kid), claimsFor(is, tid, clk.Now())))
		return err
	}
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 1, "warm")
	isOnly(t, unknown(tidA, "garbage-1"), ErrSignature)
	wantFetches(t, is, tidA, 2, "first unknown kid")
	clk.advance(4*time.Minute + 59*time.Second)
	isOnly(t, unknown(tidA, "garbage-2"), ErrSignature)
	wantFetches(t, is, tidA, 2, "second unknown kid inside 5 min")
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 2, "known kid")
	clk.advance(2 * time.Second) // 5 min 1 s after the first miss
	isOnly(t, unknown(tidA, "garbage-3"), ErrSignature)
	wantFetches(t, is, tidA, 3, "unknown kid after 5 min")
	// The budget is per tenant: B's first miss refetches though A's is spent.
	mustValidate(t, v, good(t, is, tidB, clk.Now()))
	isOnly(t, unknown(tidB, "garbage-4"), ErrSignature)
	wantFetches(t, is, tidB, 2, "tenant B's first miss")
	// A cold cache: the fetch that fills it is the one lookup; no second.
	cold := newValidator(t, is, clk)
	before := is.fetches(tidA)
	_, err := cold.Validate(bg, mint(t, testKey(t), hdr("garbage-5"), claimsFor(is, tidA, clk.Now())))
	isOnly(t, err, ErrSignature)
	wantFetches(t, is, tidA, before+1, "cold cache, unknown kid")
}

func TestRotationIsPickedUpOnAnUnknownKid(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	pub1 := &testKey(t).PublicKey
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	// Entra publishes k2 beside k1, then starts signing with it, inside the 24 h.
	k2 := newKey(t, 2048)
	is.setKeys(tidA, rsaJWK("k2", &k2.PublicKey), rsaJWK(kid1, pub1))
	mustValidate(t, v, mint(t, k2, hdr("k2"), claimsFor(is, tidA, clk.Now())))
	wantFetches(t, is, tidA, 2, "rotated")
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 2, "old key, still published")
	// k1 is withdrawn. After the next refresh it no longer validates: the new
	// set replaces the old one, it is not merged into it.
	is.setKeys(tidA, rsaJWK("k2", &k2.PublicKey))
	clk.advance(24*time.Hour + time.Second)
	_, err := v.Validate(bg, good(t, is, tidA, clk.Now()))
	isOnly(t, err, ErrSignature)
	mustValidate(t, v, mint(t, k2, hdr("k2"), claimsFor(is, tidA, clk.Now())))
}

func TestTheKeySetIsCachedFor24Hours(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 1, "two validations")
	clk.advance(24*time.Hour - time.Second)
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 1, "one second short of 24 h")
	clk.advance(2 * time.Second)
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 2, "one second past 24 h")
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 2, "after the refresh")
}

func TestConcurrentMissesFetchOnce(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	is.setDelay(200 * time.Millisecond)
	tok := good(t, is, tidA, clk.Now())
	start := make(chan struct{})
	errs := make([]error, 20)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = v.Validate(bg, tok)
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	wantFetches(t, is, tidA, 1, "20 goroutines, cold cache")
	// A caller that gives up does not cancel the fetch the others share: the
	// fetch completes and is cached.
	is.setDelay(time.Second)
	ctx, cancel := context.WithTimeout(bg, 50*time.Millisecond)
	defer cancel()
	_, err := v.Validate(ctx, good(t, is, tidB, clk.Now()))
	isOnly(t, err, ErrKeySetUnavailable)
	mustValidate(t, v, good(t, is, tidB, clk.Now()))
	wantFetches(t, is, tidB, 1, "after an abandoned wait")
}

func TestAKeySetOutageIsNotARejection(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	outage := func(v *JWKSValidator, tok, when string) {
		t.Helper()
		_, err := v.Validate(bg, tok)
		if err == nil {
			t.Fatalf("%s: accepted", when)
		}
		// isOnly also asserts errors.Is is false for all ten rejections.
		isOnly(t, err, ErrKeySetUnavailable)
	}
	for _, status := range []int{500, 503, 404, 302} {
		is.fail(tidA, status)
		outage(v, good(t, is, tidA, clk.Now()), http.StatusText(status))
	}
	// Recovery: a failed fetch is not cached.
	is.fail(tidA, 0)
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	// A set of no usable keys is no key set.
	is.setKeys(tidB)
	outage(v, good(t, is, tidB, clk.Now()), "empty key set")
	// A stale set is not used past 24 h when the refresh fails, for as long
	// as it fails: a failed refresh does not restart the 24 h (review I1b).
	clk.advance(24*time.Hour + time.Second)
	is.fail(tidA, 500)
	outage(v, good(t, is, tidA, clk.Now()), "stale set, refresh failing")
	outage(v, good(t, is, tidA, clk.Now()), "stale set, refresh still failing")
	// An unknown kid whose refetch fails cannot be judged: an outage.
	is.fail(tidA, 0)
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	is.fail(tidA, 500)
	outage(v, mint(t, testKey(t), hdr("k-new"), claimsFor(is, tidA, clk.Now())), "miss, refetch failing")
	// That failed refetch left the good set in place: a known kid still
	// validates, from the cache, with no new fetch (review I1a). A garbage
	// kid during a JWKS blip must not lock the whole tenant out.
	before := is.fetches(tidA)
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, before, "a known kid after a failed refetch")
	// The endpoint is not JSON, or not there at all.
	notJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>sign in</html>"))
	}))
	defer notJSON.Close()
	cfg := testConfig(is, clk)
	cfg.AuthorityHost, cfg.HTTPClient = notJSON.URL, notJSON.Client()
	vj, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	claims := edit(claimsFor(is, tidA, clk.Now()), map[string]any{"iss": notJSON.URL + "/" + tidA + "/v2.0"})
	outage(vj, mint(t, testKey(t), hdr(kid1), claims), "not JSON")
	closed := httptest.NewServer(http.NotFoundHandler())
	cfg.AuthorityHost, cfg.HTTPClient = closed.URL, nil
	closed.Close()
	vc, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	outage(vc, good(t, is, tidA, clk.Now()), "connection refused")
	// A body over 1 MiB is refused even when its JSON is whole inside the
	// first MiB: a valid set, then a MiB of spaces. (A body cut off mid-JSON
	// would fail as "not JSON" with or without the cap, and prove nothing.)
	set, err := json.Marshal(map[string]any{"keys": []any{rsaJWK(kid1, &testKey(t).PublicKey)}})
	if err != nil {
		t.Fatal(err)
	}
	huge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(set)
		_, _ = w.Write([]byte(strings.Repeat(" ", 1<<20)))
	}))
	defer huge.Close()
	cfg.AuthorityHost, cfg.HTTPClient = huge.URL, huge.Client()
	vb, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	hugeClaims := edit(claimsFor(is, tidA, clk.Now()), map[string]any{"iss": huge.URL + "/" + tidA + "/v2.0"})
	outage(vb, mint(t, testKey(t), hdr(kid1), hugeClaims), "key set over 1 MiB")
	// The key set comes from the authority and nowhere else. A redirect, even
	// to a server holding a usable set, is an outage, on the default client
	// and on a caller's client alike; the caller's client is not changed
	// (review M4).
	is.fail(tidA, 0)
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, is.srv.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirect.Close()
	redirClaims := edit(claimsFor(is, tidA, clk.Now()), map[string]any{"iss": redirect.URL + "/" + tidA + "/v2.0"})
	for _, tc := range []struct {
		name   string
		client *http.Client
	}{{"the default client", nil}, {"a caller's client", redirect.Client()}} {
		cfg.AuthorityHost, cfg.HTTPClient = redirect.URL, tc.client
		vr, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		outage(vr, mint(t, testKey(t), hdr(kid1), redirClaims), "a redirect, "+tc.name)
	}
	if redirect.Client().CheckRedirect != nil {
		t.Error("New changed the caller's client")
	}
	// A key-set server that never answers, behind a client with no timeout
	// (httptest's has none): the validator's own cap ends the wait.
	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer hung.Close()
	defer close(release) // runs first: the handler returns, then Close
	cfg.AuthorityHost, cfg.HTTPClient = hung.URL, hung.Client()
	vh, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	vh.fetchTimeout = 200 * time.Millisecond
	tok := good(t, is, tidA, clk.Now())
	done := make(chan error, 1)
	go func() {
		_, err := vh.Validate(bg, tok)
		done <- err
	}()
	select {
	case err := <-done:
		isOnly(t, err, ErrKeySetUnavailable)
	case <-time.After(5 * time.Second):
		t.Fatal("a key-set server that never answers hung Validate")
	}
}

func TestReadyFetchesEveryTenant(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	v := newValidator(t, is, clk)
	if err := v.Ready(bg); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	wantFetches(t, is, tidA, 1, "Ready")
	wantFetches(t, is, tidB, 1, "Ready")
	wantFetches(t, is, tidC, 0, "Ready (C is not enrolled)")
	if err := v.Ready(bg); err != nil {
		t.Fatalf("Ready again: %v", err)
	}
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	wantFetches(t, is, tidA, 1, "Ready twice, then a validation")
	// One tenant failing is not ready, and says why.
	is2 := newIssuer(t)
	v2 := newValidator(t, is2, clk)
	is2.fail(tidB, 503)
	err := v2.Ready(bg)
	if err == nil {
		t.Fatal("Ready with tenant B's key set down: nil")
	}
	isOnly(t, err, ErrKeySetUnavailable)
	wantFetches(t, is2, tidA, 1, "Ready, B failing")
	is2.fail(tidB, 0)
	if err := v2.Ready(bg); err != nil {
		t.Fatalf("Ready after B recovers: %v", err)
	}
	wantFetches(t, is2, tidA, 1, "Ready after recovery")
	wantFetches(t, is2, tidB, 2, "Ready after recovery")
	// B failing first in the list, A after it: still not ready.
	is3 := newIssuer(t)
	cfg := testConfig(is3, clk)
	cfg.TenantIDs = []string{tidB, tidA}
	v3, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	is3.fail(tidA, 500)
	isOnly(t, v3.Ready(bg), ErrKeySetUnavailable)
}

// Added by task 16's build: the part states the key filter (kty RSA, use sig or
// absent, 2048 bits) but no test of the table reached it.
func TestOnlyRSASigningKeysOfAtLeast2048BitsAreUsed(t *testing.T) {
	is, clk := newIssuer(t), newClock()
	pub := &testKey(t).PublicKey
	small := newKey(t, 1024)
	enc := rsaJWK("enc", pub)
	enc["use"] = "enc"
	ec := rsaJWK("ec", pub)
	ec["kty"] = "EC"
	noUse := rsaJWK("no-use", pub)
	delete(noUse, "use")
	pss := rsaJWK("pss", pub)
	pss["alg"] = "PS256"
	rs := rsaJWK("rs", pub)
	rs["alg"] = "RS256"
	// A duplicate kid: the first key published under it wins, so a later
	// entry cannot shadow it.
	dup := rsaJWK(kid1, &newKey(t, 2048).PublicKey)
	// An exponent over four bytes, which Int64 would truncate to 65537.
	wide := rsaJWK("wide-e", pub)
	wide["e"] = b64(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(int64(pub.E))).Bytes())
	// An exponent of 1, under which a "signature" is its own encoding.
	unit := rsaJWK("e-1", pub)
	unit["e"] = b64([]byte{1})
	is.setKeys(tidA, rsaJWK(kid1, pub), rsaJWK("small", &small.PublicKey), enc, ec, noUse, pss, rs, dup, wide, unit)
	v := newValidator(t, is, clk)
	claims := claimsFor(is, tidA, clk.Now())
	mustValidate(t, v, mint(t, testKey(t), hdr(kid1), claims))
	mustValidate(t, v, mint(t, testKey(t), hdr("no-use"), claims))
	mustValidate(t, v, mint(t, testKey(t), hdr("rs"), claims))
	// Anyone can "sign" for the e = 1 key by writing out the PKCS #1 v1.5
	// encoding. Go 1.23's VerifyPKCS1v15 checks no exponent and would accept
	// it (review M2); the key must never reach the verifier.
	forged := forgeForUnitExponent(t, pub.N, hdr("e-1"), claims)
	// The forgery is real: it is what a genuine signature over the same
	// input raises to, s^65537 mod n, so the check below tests the key
	// filter and not a malformed token.
	genuine := mint(t, testKey(t), hdr("e-1"), claims)
	sig, err := base64.RawURLEncoding.DecodeString(genuine[strings.LastIndexByte(genuine, '.')+1:])
	if err != nil {
		t.Fatal(err)
	}
	em := new(big.Int).Exp(new(big.Int).SetBytes(sig), big.NewInt(int64(pub.E)), pub.N)
	if want := b64(em.FillBytes(make([]byte, len(sig)))); forged[strings.LastIndexByte(forged, '.')+1:] != want {
		t.Fatal("forgeForUnitExponent does not build the PKCS #1 v1.5 encoding")
	}
	if _, err := v.Validate(bg, forged); err == nil {
		t.Fatal("a token forged for an e = 1 key: accepted")
	} else {
		isOnly(t, err, ErrSignature)
	}
	// rsaKey refuses every key that some supported toolchain's crypto/rsa
	// refuses or mis-handles, so what is trusted never depends on the Go
	// version. Entra publishes e = 65537 ("AQAB").
	n := b64(pub.N.Bytes())
	for _, tc := range []struct {
		name, n, e string
		ok         bool
	}{
		{"e = 65537", n, "AQAB", true},
		{"e = 3", n, b64([]byte{3}), true},
		{"e = 2^31-1", n, b64([]byte{0x7f, 0xff, 0xff, 0xff}), true},
		{"e = 0", n, b64([]byte{0}), false},
		{"e = 1", n, b64([]byte{1}), false},
		{"e = 2", n, b64([]byte{2}), false},
		{"an even e", n, b64(big.NewInt(1 << 16).Bytes()), false},
		{"e = 2^31+1", n, b64([]byte{0x80, 0, 0, 1}), false},
		{"an even modulus", b64(new(big.Int).Add(pub.N, big.NewInt(1)).Bytes()), "AQAB", false},
	} {
		if _, ok := rsaKey(tc.n, tc.e); ok != tc.ok {
			t.Errorf("rsaKey, %s: ok = %v, want %v", tc.name, ok, tc.ok)
		}
	}
	for kid, key := range map[string]any{"small": small, "enc": nil, "ec": nil, "pss": nil, "wide-e": nil} {
		signer := testKey(t)
		if key != nil {
			signer = small
		}
		_, err := v.Validate(bg, mint(t, signer, hdr(kid), claims))
		if err == nil {
			t.Fatalf("kid %q: accepted", kid)
		}
		isOnly(t, err, ErrSignature)
	}
	// A key set holding only unusable keys is no key set at all.
	is.setKeys(tidB, rsaJWK("small", &small.PublicKey), enc)
	_, err = v.Validate(bg, mint(t, small, hdr("small"), claimsFor(is, tidB, clk.Now())))
	isOnly(t, err, ErrKeySetUnavailable)
}
