package entra

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	// A stale set is not used past 24 h when the refresh fails.
	clk.advance(24*time.Hour + time.Second)
	is.fail(tidA, 500)
	outage(v, good(t, is, tidA, clk.Now()), "stale set, refresh failing")
	// An unknown kid whose refetch fails cannot be judged: an outage.
	is.fail(tidA, 0)
	mustValidate(t, v, good(t, is, tidA, clk.Now()))
	is.fail(tidA, 500)
	outage(v, mint(t, testKey(t), hdr("k-new"), claimsFor(is, tidA, clk.Now())), "miss, refetch failing")
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
	is.setKeys(tidA, rsaJWK(kid1, pub), rsaJWK("small", &small.PublicKey), enc, ec, noUse, pss, rs)
	v := newValidator(t, is, clk)
	claims := claimsFor(is, tidA, clk.Now())
	mustValidate(t, v, mint(t, testKey(t), hdr(kid1), claims))
	mustValidate(t, v, mint(t, testKey(t), hdr("no-use"), claims))
	mustValidate(t, v, mint(t, testKey(t), hdr("rs"), claims))
	for kid, key := range map[string]any{"small": small, "enc": nil, "ec": nil, "pss": nil} {
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
	_, err := v.Validate(bg, mint(t, small, hdr("small"), claimsFor(is, tidB, clk.Now())))
	isOnly(t, err, ErrKeySetUnavailable)
}
