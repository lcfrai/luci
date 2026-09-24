package entra

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const (
	// keySetTTL is how long a tenant's fetched key set is trusted.
	keySetTTL = 24 * time.Hour
	// missInterval bounds the refetches an unknown kid can cause, per tenant,
	// so a caller sending garbage kids cannot hammer the JWKS endpoint.
	missInterval = 5 * time.Minute
	// maxKeySetBytes bounds the JWKS body. Entra's is a few kilobytes.
	maxKeySetBytes = 1 << 20
	// minModulusBits: a smaller RSA key is never used.
	minModulusBits = 2048
)

// tenantKeys is one tenant's cache entry. Guarded by JWKSValidator.mu.
type tenantKeys struct {
	keys      map[string]*rsa.PublicKey // nil until the first successful fetch
	fetchedAt time.Time
	missAt    time.Time  // when an unknown kid last caused a refetch
	call      *fetchCall // the fetch in flight, or nil
}

// fetchCall is one in-flight fetch that every concurrent caller shares.
type fetchCall struct {
	done chan struct{}
	err  error
}

// key returns tid's key for kid, fetching the key set when it is missing or
// older than keySetTTL, and refetching once per missInterval for a kid the
// cached set does not hold. tid must be enrolled.
func (v *JWKSValidator) key(ctx context.Context, tid, kid string) (*rsa.PublicKey, error) {
	fetched := false
	v.mu.Lock()
	t := v.sets[tid]
	if t.keys == nil || !v.now().Before(t.fetchedAt.Add(keySetTTL)) {
		v.mu.Unlock()
		if err := v.refresh(ctx, tid); err != nil {
			return nil, err
		}
		fetched = true
		v.mu.Lock()
	}
	if k, ok := t.keys[kid]; ok {
		v.mu.Unlock()
		return k, nil
	}
	now := v.now()
	// The set was fetched for this very call: refetching now learns nothing.
	if fetched || (!t.missAt.IsZero() && now.Sub(t.missAt) < missInterval) {
		v.mu.Unlock()
		return nil, fmt.Errorf("%w: kid not in the tenant's key set", ErrSignature)
	}
	t.missAt = now
	v.mu.Unlock()
	// A new kid may be a rotation inside the 24 h: look once more.
	if err := v.refresh(ctx, tid); err != nil {
		return nil, err
	}
	v.mu.Lock()
	k, ok := t.keys[kid]
	v.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%w: kid not in the tenant's key set after a refetch", ErrSignature)
	}
	return k, nil
}

// refresh fetches tid's key set, sharing a fetch already in flight. The fetch
// runs detached from ctx, so a caller that gives up does not fail the others
// waiting on it; the caller itself stops waiting when ctx ends.
func (v *JWKSValidator) refresh(ctx context.Context, tid string) error {
	v.mu.Lock()
	t := v.sets[tid]
	c := t.call
	if c == nil {
		c = &fetchCall{done: make(chan struct{})}
		t.call = c
		go v.fetch(context.WithoutCancel(ctx), tid, t, c)
	}
	v.mu.Unlock()
	select {
	case <-c.done:
		return c.err
	case <-ctx.Done():
		return fmt.Errorf("%w: gave up waiting for the key set: %w", ErrKeySetUnavailable, ctx.Err())
	}
}

func (v *JWKSValidator) fetch(ctx context.Context, tid string, t *tenantKeys, c *fetchCall) {
	// The client's own timeout is not trusted to exist on a caller's client.
	ctx, cancel := context.WithTimeout(ctx, v.fetchTimeout)
	defer cancel()
	keys, err := v.download(ctx, tid)
	v.mu.Lock()
	if err == nil {
		t.keys, t.fetchedAt = keys, v.now()
	}
	t.call = nil
	c.err = err
	v.mu.Unlock()
	close(c.done)
}

// download GETs and parses one tenant's key set. Every failure wraps
// ErrKeySetUnavailable; none carries a token, because none has seen one.
func (v *JWKSValidator) download(ctx context.Context, tid string) (map[string]*rsa.PublicKey, error) {
	u := v.authority + "/" + tid + "/discovery/v2.0/keys"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrKeySetUnavailable, u, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeySetUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: GET %s answered %d", ErrKeySetUnavailable, u, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxKeySetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: GET %s: reading the body: %v", ErrKeySetUnavailable, u, err)
	}
	if len(body) > maxKeySetBytes {
		return nil, fmt.Errorf("%w: GET %s: key set over %d bytes", ErrKeySetUnavailable, u, maxKeySetBytes)
	}
	keys, err := parseKeySet(body)
	if err != nil {
		return nil, fmt.Errorf("%w: GET %s: %v", ErrKeySetUnavailable, u, err)
	}
	return keys, nil
}

type jsonWebKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// parseKeySet keeps the keys a token may be verified with: kty RSA, use
// absent or "sig", alg absent or RS256, a kid, and a modulus of at least 2048
// bits. Anything else is skipped, not an error, so one odd key cannot take a
// tenant down. A set with no usable key is an error: it can verify nothing.
func parseKeySet(body []byte) (map[string]*rsa.PublicKey, error) {
	var set struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, errors.New("the key set is not JSON")
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") || (k.Alg != "" && k.Alg != "RS256") || k.Kid == "" {
			continue
		}
		if _, dup := keys[k.Kid]; dup {
			continue // the first key published under a kid wins
		}
		pub, ok := rsaKey(k.N, k.E)
		if !ok {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no usable RS256 signing key among %d keys", len(set.Keys))
	}
	return keys, nil
}

// rsaKey decodes a JWK's two unpadded base64url integers (RFC 7518 §6.3.1).
// An exponent over four bytes is refused here: Int64 would silently truncate
// it into some other, valid-looking exponent.
//
// So is every key that some supported toolchain's crypto/rsa refuses or
// mis-handles, so that what is trusted never depends on the Go version: an
// exponent under 3, even, or over 2^31-1, and an even modulus. Go 1.23 (the
// module's go line and CI's pin) verifies PKCS #1 v1.5 without checking the
// exponent, and with e = 1 a "signature" is its own encoding: anyone could
// forge one. Entra publishes e = 65537.
func rsaKey(n, e string) (*rsa.PublicKey, bool) {
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, false
	}
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil || len(eb) == 0 || len(eb) > 4 {
		return nil, false
	}
	mod := new(big.Int).SetBytes(nb)
	if mod.BitLen() < minModulusBits || mod.Bit(0) == 0 {
		return nil, false
	}
	exp := new(big.Int).SetBytes(eb).Int64()
	if exp < 3 || exp%2 == 0 || exp > 1<<31-1 {
		return nil, false
	}
	return &rsa.PublicKey{N: mod, E: int(exp)}, true
}

// Ready fetches the key set of every enrolled tenant not yet fetched and
// returns the first failure (wrapping ErrKeySetUnavailable). The broker's
// /readyz calls it; the Hub's /readyz may (ADR-H1 §5.7, 67609ab :528-530).
// Tenants are fetched concurrently; the first failure is in TenantIDs order.
func (v *JWKSValidator) Ready(ctx context.Context) error {
	errs := make([]error, len(v.tenants))
	var wg sync.WaitGroup
	for i, tid := range v.tenants {
		v.mu.Lock()
		fetched := v.sets[tid].keys != nil
		v.mu.Unlock()
		if fetched {
			continue
		}
		wg.Add(1)
		go func(i int, tid string) {
			defer wg.Done()
			errs[i] = v.refresh(ctx, tid)
		}(i, tid)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
