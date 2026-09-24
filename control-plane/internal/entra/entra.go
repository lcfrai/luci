// Package entra validates Microsoft Entra ID v2 access tokens for a resource API.
// Standard library only: RS256 over crypto/rsa, JWKS over net/http.
//
// It is the one validator the broker (control-plane/cmd/broker) and the Hub
// share (HA-1; ADR-H1 §4.1): one package, so the two cannot drift into
// accepting different tokens. There is deliberately no JOSE library: one would
// accept algorithms this package must refuse, and not having it removes the
// key-confusion class rather than configuring it away.
package entra

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct {
	// AuthorityHost is "https://login.microsoftonline.com" in every deployed
	// environment, or devissuer's base URL locally. The tenant id is appended.
	AuthorityHost string
	// TenantIDs are the enrolled tenants. A token from any other is ErrTenant.
	TenantIDs []string
	// Audience is the resource API's client id, a GUID: a v2 token's aud.
	Audience string
	// RequiredScope must appear in scp, e.g. "Hub.Access" or "Sandboxes.Access".
	RequiredScope string
	// AllowedClients is the azp allowlist: the desktop's client id.
	AllowedClients []string
	// ClockSkew defaults to 120 s.
	ClockSkew time.Duration
	// AllowInsecureAuthority permits an http AuthorityHost, and only a loopback one.
	AllowInsecureAuthority bool
	HTTPClient             *http.Client // default: 10 s timeout
	Now                    func() time.Time
}

// Claims is what a validated token says. It holds no token text.
type Claims struct {
	TenantID          string   // tid
	ObjectID          string   // oid
	ClientID          string   // azp
	Name              string   // name
	PreferredUsername string   // preferred_username
	Email             string   // email, when the optional claim is configured
	Scopes            []string // scp, split on spaces
	Roles             []string // roles
	ExpiresAt         time.Time
}

// Key is "tid/oid", the stable identity. Never sub (pairwise per app), never
// preferred_username (mutable) — ADR-H1 §5.2 (67609ab :366).
func (c Claims) Key() string { return c.TenantID + "/" + c.ObjectID }

type Validator interface {
	Validate(ctx context.Context, bearer string) (Claims, error)
}

// Every rejection wraps exactly one of these. The Hub maps each to details.reason.
var (
	ErrMalformed   = errors.New("entra: malformed token")       // "malformed"
	ErrSignature   = errors.New("entra: bad signature")         // "signature"
	ErrExpired     = errors.New("entra: token expired")         // "expired"
	ErrNotYetValid = errors.New("entra: token not yet valid")   // "not_yet_valid"
	ErrIssuer      = errors.New("entra: wrong issuer")          // "issuer"
	ErrAudience    = errors.New("entra: wrong audience")        // "audience"
	ErrScope       = errors.New("entra: required scope absent") // "scope"
	ErrClient      = errors.New("entra: client not allowed")    // "client"
	ErrVersion     = errors.New("entra: not a v2 token")        // "version"
	ErrTenant      = errors.New("entra: tenant not enrolled")   // 403 tenant_not_enrolled
)

// ErrKeySetUnavailable: the tenant's JWKS could not be fetched. NOT a
// rejection of the token — nobody could be authenticated — so the broker
// answers 503, not 401. It is the only error Validate returns that is not one
// of the ten above.
var ErrKeySetUnavailable = errors.New("entra: key set unavailable")

const (
	defaultSkew    = 120 * time.Second
	defaultTimeout = 10 * time.Second
)

// JWKSValidator implements Validator. Safe for concurrent use.
type JWKSValidator struct {
	authority string // scheme://host, no trailing slash
	tenants   []string
	enrolled  map[string]bool
	audience  string
	scope     string
	clients   map[string]bool
	skew      time.Duration
	client    *http.Client
	now       func() time.Time
	// fetchTimeout caps one key-set fetch whatever client is configured: a
	// caller's client may have no timeout, and the fetch outlives its caller.
	fetchTimeout time.Duration

	mu   sync.Mutex
	sets map[string]*tenantKeys // one per enrolled tenant, made by New
}

var _ Validator = (*JWKSValidator)(nil)

// New checks cfg and returns a validator. It errors on an unusable Config and
// fetches nothing: the first Validate or Ready of a tenant fetches its keys.
//
// GUIDs in cfg are trimmed and lower-cased, because Entra writes tid, aud and
// azp in lower case and a comparison against an upper-case configuration
// would refuse every token.
//
// The HTTP client, the default or cfg.HTTPClient, is used through a copy that
// follows no redirect.
func New(cfg Config) (*JWKSValidator, error) {
	authority, err := parseAuthority(cfg.AuthorityHost, cfg.AllowInsecureAuthority)
	if err != nil {
		return nil, err
	}
	v := &JWKSValidator{
		authority:    authority,
		enrolled:     map[string]bool{},
		clients:      map[string]bool{},
		skew:         cfg.ClockSkew,
		client:       cfg.HTTPClient,
		now:          cfg.Now,
		fetchTimeout: defaultTimeout,
		sets:         map[string]*tenantKeys{},
	}
	if len(cfg.TenantIDs) == 0 {
		return nil, errors.New("entra: config: no TenantIDs")
	}
	for _, raw := range cfg.TenantIDs {
		tid := normGUID(raw)
		if !isGUID(tid) {
			return nil, fmt.Errorf("entra: config: tenant id %q is not a GUID", raw)
		}
		if !v.enrolled[tid] {
			v.enrolled[tid] = true
			v.tenants = append(v.tenants, tid)
			v.sets[tid] = &tenantKeys{}
		}
	}
	if v.audience = normGUID(cfg.Audience); !isGUID(v.audience) {
		return nil, fmt.Errorf("entra: config: Audience %q is not a client id GUID (a v2 token's aud)", cfg.Audience)
	}
	v.scope = cfg.RequiredScope
	if v.scope == "" || strings.ContainsAny(v.scope, " \t\r\n") {
		return nil, fmt.Errorf("entra: config: RequiredScope %q must be one scope", cfg.RequiredScope)
	}
	if len(cfg.AllowedClients) == 0 {
		return nil, errors.New("entra: config: no AllowedClients")
	}
	for _, raw := range cfg.AllowedClients {
		id := normGUID(raw)
		if !isGUID(id) {
			return nil, fmt.Errorf("entra: config: allowed client %q is not a GUID", raw)
		}
		v.clients[id] = true
	}
	if v.skew < 0 {
		return nil, fmt.Errorf("entra: config: negative ClockSkew %v", cfg.ClockSkew)
	}
	if v.skew == 0 {
		v.skew = defaultSkew
	}
	if v.client == nil {
		v.client = &http.Client{Timeout: defaultTimeout}
	} else {
		c := *v.client // a copy: the caller's client is not changed
		v.client = &c
	}
	// The key set is the trust anchor and comes from the authority alone,
	// whoever built the client: a redirect is answered as the 3xx it is, a
	// non-200 answer, so ErrKeySetUnavailable.
	v.client.CheckRedirect = refuseRedirect
	if v.now == nil {
		v.now = time.Now
	}
	return v, nil
}

// refuseRedirect makes a client hand back a 3xx as it is, unfollowed.
func refuseRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// parseAuthority returns scheme://host for an https authority, or for an http
// one when insecure is set and the host is a loopback IP literal. A name, even
// "localhost", is refused for http: the key set is the trust anchor, and a name
// is one resolver away from somewhere else.
func parseAuthority(raw string, insecure bool) (string, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || u.Host == "" || u.Opaque != "" {
		return "", fmt.Errorf("entra: config: AuthorityHost %q is not an absolute URL", raw)
	}
	if u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("entra: config: AuthorityHost %q must be scheme://host only", raw)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !insecure {
			return "", fmt.Errorf("entra: config: AuthorityHost %q is http; set AllowInsecureAuthority for a loopback devissuer", raw)
		}
		if ip := net.ParseIP(u.Hostname()); ip == nil || !ip.IsLoopback() {
			return "", fmt.Errorf("entra: config: AuthorityHost %q: http is allowed only on a loopback IP", raw)
		}
	default:
		return "", fmt.Errorf("entra: config: AuthorityHost %q: scheme must be https", raw)
	}
	return u.Scheme + "://" + strings.ToLower(u.Host), nil
}

func normGUID(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// isGUID reports whether s is a lower-case 8-4-4-4-12 hex GUID, the form Entra
// writes tid, oid, aud and azp in.
func isGUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
	}
	return true
}
