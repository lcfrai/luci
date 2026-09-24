package entra

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Fictional identifiers. Every one is a lowercase GUID, as Entra writes them.
const (
	tidA      = "0b6c1a3e-5f2d-4c8a-9e71-3d4f5a6b7c8d" // enrolled
	tidB      = "1c7d2b4f-6a3e-4d9b-8f82-4e5a6b7c8d9e" // enrolled
	tidC      = "2d8e3c5a-7b4f-4eac-9a93-5f6b7c8d9eaf" // NOT enrolled
	brokerAPI = "3e9f4d6b-8c5a-4fbd-8ba4-6a7c8d9eafb0" // the broker's resource API
	hubAPI    = "4fa05e7c-9d6b-4ace-9cb5-7b8d9eafb0c1" // the Hub's resource API
	desktop   = "5ab16f8d-ae7c-4bdf-8dc6-8c9eafb0c1d2" // the desktop's public client
	otherApp  = "6bc2709e-bf8d-4ce0-9ed7-9dafb0c1d2e3" // some other app in the tenant
	userOID   = "7cd3810f-c09e-4df1-8fe8-aeb0c1d2e3f4"
	testScope = "Sandboxes.Access"
	kid1      = "k1"
)

// t0 is every test's starting clock: whole seconds, so exp and nbf arithmetic
// in the skew test is exact.
var t0 = time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)

// One 2048-bit key per test binary. Generating one costs up to a second on a
// loaded laptop; every test shares it.
var (
	keyOnce sync.Once
	key1    *rsa.PrivateKey
)

func testKey(t testing.TB) *rsa.PrivateKey {
	t.Helper()
	keyOnce.Do(func() {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		key1 = k
	})
	return key1
}

func newKey(t testing.TB, bits int) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// rsaJWK is the JWKS entry Entra publishes for a signing key.
func rsaJWK(kid string, pub *rsa.PublicKey) map[string]any {
	return map[string]any{
		"kty": "RSA",
		"use": "sig",
		"kid": kid,
		"n":   b64(pub.N.Bytes()),
		"e":   b64(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// issuer serves /{tid}/discovery/v2.0/keys on loopback and counts requests.
type issuer struct {
	srv    *httptest.Server
	mu     sync.Mutex
	sets   map[string][]map[string]any
	status map[string]int
	delay  time.Duration
	hits   map[string]int
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	is := &issuer{
		sets:   map[string][]map[string]any{},
		status: map[string]int{},
		hits:   map[string]int{},
	}
	pub := &testKey(t).PublicKey
	for _, tid := range []string{tidA, tidB, tidC} {
		is.sets[tid] = []map[string]any{rsaJWK(kid1, pub)}
	}
	is.srv = httptest.NewServer(http.HandlerFunc(is.serve))
	t.Cleanup(is.srv.Close)
	return is
}

func (is *issuer) serve(w http.ResponseWriter, r *http.Request) {
	tid, rest, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if !ok || rest != "discovery/v2.0/keys" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	is.mu.Lock()
	is.hits[tid]++
	keys, known := is.sets[tid]
	status, delay := is.status[tid], is.delay
	is.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	if !known {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// A failing status still carries the tenant's usable key set, so a test
	// of it passes only if the status alone is refused. An empty body would
	// fail as "not JSON" with or without the status check (review I2).
	if status != 0 {
		w.WriteHeader(status)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
}

func (is *issuer) fetches(tid string) int {
	is.mu.Lock()
	defer is.mu.Unlock()
	return is.hits[tid]
}

func (is *issuer) total() int {
	is.mu.Lock()
	defer is.mu.Unlock()
	n := 0
	for _, h := range is.hits {
		n += h
	}
	return n
}

func (is *issuer) setKeys(tid string, keys ...map[string]any) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.sets[tid] = keys
}

// fail makes tid's endpoint answer status, with its usable key set as the
// body; 0 restores 200.
func (is *issuer) fail(tid string, status int) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.status[tid] = status
}

func (is *issuer) setDelay(d time.Duration) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.delay = d
}

func (is *issuer) iss(tid string) string { return is.srv.URL + "/" + tid + "/v2.0" }

// clock is a settable Now.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: t0} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// testConfig is the broker's configuration against the test issuer.
func testConfig(is *issuer, clk *clock) Config {
	return Config{
		AuthorityHost:          is.srv.URL,
		TenantIDs:              []string{tidA, tidB},
		Audience:               brokerAPI,
		RequiredScope:          testScope,
		AllowedClients:         []string{desktop},
		AllowInsecureAuthority: true,
		HTTPClient:             is.srv.Client(),
		Now:                    clk.Now,
	}
}

func newValidator(t *testing.T, is *issuer, clk *clock) *JWKSValidator {
	t.Helper()
	v, err := New(testConfig(is, clk))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

// claimsFor is a valid v2 access token's payload for tid at now.
func claimsFor(is *issuer, tid string, now time.Time) map[string]any {
	return map[string]any{
		"ver":                "2.0",
		"iss":                is.iss(tid),
		"aud":                brokerAPI,
		"tid":                tid,
		"oid":                userOID,
		"sub":                "pairwise-sub-for-the-broker",
		"azp":                desktop,
		"scp":                testScope,
		"name":               "Ada Lovelace",
		"preferred_username": "ada@example.test",
		"email":              "ada.lovelace@example.test",
		"roles":              []string{"Reader"},
		"iat":                now.Add(-5 * time.Minute).Unix(),
		"nbf":                now.Add(-5 * time.Minute).Unix(),
		"exp":                now.Add(time.Hour).Unix(),
	}
}

// drop, as a value in edit's changes, deletes the claim.
var drop = struct{ drop bool }{true}

// edit copies m and applies changes.
func edit(m map[string]any, changes map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	for k, v := range changes {
		if v == drop {
			delete(out, k)
			continue
		}
		out[k] = v
	}
	return out
}

func hdr(kid string) map[string]any {
	return map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid}
}

func seg(t testing.TB, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b64(b)
}

// mint signs header.payload with key, RS256, whatever the header says.
func mint(t testing.TB, key *rsa.PrivateKey, header, claims map[string]any) string {
	t.Helper()
	input := seg(t, header) + "." + seg(t, claims)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + b64(sig)
}

// sha256DigestInfo is the DER DigestInfo prefix PKCS #1 v1.5 puts before a
// SHA-256 hash (RFC 8017 §9.2, note 1).
var sha256DigestInfo = []byte{0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20}

// forgeForUnitExponent "signs" header.payload for the RSA key (n, e = 1)
// without any private key: with e = 1, s^e mod n is s, so the signature is
// the encoding EM = 0x00 0x01 FF..FF 0x00 DigestInfo hash itself.
func forgeForUnitExponent(t testing.TB, n *big.Int, header, claims map[string]any) string {
	t.Helper()
	input := seg(t, header) + "." + seg(t, claims)
	sum := sha256.Sum256([]byte(input))
	k := (n.BitLen() + 7) / 8
	tLen := len(sha256DigestInfo) + len(sum)
	em := make([]byte, k)
	em[1] = 0x01
	for i := 2; i < k-tLen-1; i++ {
		em[i] = 0xff
	}
	copy(em[k-tLen:], sha256DigestInfo)
	copy(em[k-len(sum):], sum[:])
	return input + "." + b64(em)
}

// good is a valid token for tid at the clock's time.
func good(t testing.TB, is *issuer, tid string, now time.Time) string {
	t.Helper()
	return mint(t, testKey(t), hdr(kid1), claimsFor(is, tid, now))
}

var rejections = []error{
	ErrMalformed, ErrSignature, ErrExpired, ErrNotYetValid, ErrIssuer,
	ErrAudience, ErrScope, ErrClient, ErrVersion, ErrTenant,
}

// isOnly fails unless err wraps want and no other sentinel of the package.
func isOnly(t testing.TB, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	for _, other := range append(rejections[:len(rejections):len(rejections)], ErrKeySetUnavailable) {
		if other != want && errors.Is(err, other) {
			t.Fatalf("err = %v wraps %v as well as %v", err, other, want)
		}
	}
}
