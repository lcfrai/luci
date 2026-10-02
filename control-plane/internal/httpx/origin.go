package httpx

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// OriginPolicy decides whether a browser context may talk to this control plane.
//
// WHY NOT coder/websocket's built-in check: AcceptOptions.OriginPatterns matches
// on the Origin's HOST ONLY, ignoring the scheme, and returns nil for a MISSING
// Origin. Both are reasonable defaults for a web server and both are wrong here:
// a loopback control plane's whole threat model is a web page the user happens
// to have open. This policy compares the full origin string and treats an
// unknown origin as a hard 403 with a log line naming what was rejected and what
// is allowed - a silent 403 on a laptop is a bug report that costs an afternoon.
type OriginPolicy struct {
	allowed map[string]struct{}
	order   []string // the same set, in configured order, for the log line
	log     *slog.Logger
}

// DefaultOrigins is what a Tauri webview actually sends.
//
// HOW THIS WAS DETERMINED, because it is the field most likely to be wrong:
// tauri v2 computes the webview's origin in crates/tauri/src/manager/webview.rs.
// For a custom-scheme URL on Windows and Android it produces
// "{http|https}://{scheme}.localhost"; on macOS and Linux it leaves the custom
// scheme intact, giving "tauri://localhost". Which of http and https is chosen
// is WindowConfig.use_https_scheme, which defaults to false, and
// desktop/src-tauri/tauri.conf.json does not set it - so the Windows build sends
// http://tauri.localhost. In development the window loads build.devUrl directly,
// so the origin is http://localhost:1420.
//
// All five are allowed because the same binary serves a Windows dev run, a
// Windows packaged run and a macOS or Linux developer's machine, and an
// allowlist that is right on one of them and silently wrong on the others is
// worse than one that is explicit.
func DefaultOrigins() []string {
	return []string{
		"http://tauri.localhost",  // Tauri 2 packaged, Windows (this repo's default)
		"https://tauri.localhost", // Tauri 2 packaged, Windows with useHttpsScheme
		"tauri://localhost",       // Tauri 2 packaged, macOS and Linux
		"http://localhost:1420",   // vite dev server, tauri.conf.json build.devUrl
		"http://127.0.0.1:1420",   // the same dev server reached by address
	}
}

// NewOriginPolicy returns a policy allowing exactly these origins. An empty
// slice means DefaultOrigins.
//
// A non-empty slice whose entries all normalise away leaves the allowlist EMPTY
// and logs a warning. Falling back to the defaults there would silently restore
// five origins the operator did not ask for, and LUCI_ALLOWED_ORIGINS
// exists to lock the surface down.
func NewOriginPolicy(allowed []string, log *slog.Logger) *OriginPolicy {
	if log == nil {
		log = slog.Default()
	}
	in := allowed
	if len(in) == 0 {
		in = DefaultOrigins()
	}
	p := &OriginPolicy{allowed: make(map[string]struct{}, len(in)), log: log}
	for _, raw := range in {
		origin := normaliseOrigin(raw)
		if origin == "" {
			continue
		}
		if _, dup := p.allowed[origin]; dup {
			continue
		}
		p.allowed[origin] = struct{}{}
		p.order = append(p.order, origin)
	}
	if len(p.order) == 0 {
		log.Warn("origin allowlist is empty; every browser request will be rejected",
			"configured", len(allowed))
	}
	return p
}

// normaliseOrigin makes comparison total without making it loose. Scheme and
// host are case-insensitive per RFC 6454 and an origin has no path, so lowering
// the whole string cannot change what it means; the trailing slash and the
// surrounding space are what an operator pastes out of an address bar.
func normaliseOrigin(s string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "/"))
}

// Allow reports whether an Origin header value is acceptable. An EMPTY origin is
// allowed: curl, the Go test client and every non-browser tool send none, and
// the attack this defends against is a browser, which always sends one.
//
// THE INVARIANT THAT ALLOWANCE RESTS ON: no route behind this middleware may
// ever have a side-effecting GET. The Host guard below stops the REBINDING
// variant of a no-Origin attack, where a page resolves evil.example to 127.0.0.1
// and the request arrives carrying Host: evil.example. It does not stop the
// DIRECT variant: a page served from anywhere can emit
// <img src="http://127.0.0.1:8730/...">, the browser sets Host: 127.0.0.1:8730
// itself so the Host guard passes, and the request carries no Origin so this
// function passes it too. It reaches the handler.
//
// That is harmless today because of what the routes are, which was checked and
// not assumed: every state-changing verb in contract 3.1 is POST, PATCH or
// DELETE, and a browser sends Origin on all three - including an HTML form post,
// which is the one state-changing vector that needs no preflight. The attacking
// page cannot read any reply either, because a JSON body pulled in through <img>
// or <script> is unreadable and no Access-Control-Allow-Origin is echoed to it.
//
// So the exposure is entirely a property of the route table, and a side-effecting
// GET added later would open a real CSRF hole with nothing in this file changing
// to reveal it. Such a route needs its own defence - a token, or an Origin that
// is present AND allowed - rather than this function's answer.
func (p *OriginPolicy) Allow(origin string) bool {
	if origin == "" {
		return true
	}
	_, ok := p.allowed[normaliseOrigin(origin)]
	return ok
}

// Allowed returns the effective allowlist, so task 13 can print it at startup.
func (p *OriginPolicy) Allowed() []string {
	out := make([]string, len(p.order))
	copy(out, p.order)
	return out
}

// LoopbackHost reports whether an HTTP Host header names this machine's
// loopback interface.
//
// This is the DNS-rebinding defence and it is the check that survives an attack
// carrying no Origin at all: a page at http://evil.example can point that name
// at 127.0.0.1, and the <img> and <script src> requests it emits then reach this
// process with no Origin header and Host: evil.example. Nothing else sees them.
//
// It takes a Host HEADER, which may carry a port and may be a bracketed IPv6
// literal. Task 13's bind refusal splits LUCI_ADDR itself before calling
// this.
func LoopbackHost(hostHeader string) bool {
	h := strings.TrimSpace(hostHeader)
	if h == "" {
		// No browser omits Host, so refusing costs nothing legitimate, and
		// "unknown" is not a value this guard may read as loopback.
		return false
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	} else {
		// No port: a bare name or address, or a bracketed IPv6 literal.
		h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	}
	h = strings.TrimSuffix(h, ".") // the fully-qualified form, "localhost."
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// The desktop authenticates to its loopback management gateway with a bearer
// token. The browser must be able to preflight both that header and JSON bodies.
// This grant is only sent after the exact origin and loopback Host checks.
const (
	corsMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	corsHeaders = "Content-Type, Authorization"
	corsMaxAge  = "600"
)

// Middleware rejects a request whose Host is not loopback with 403
// forbidden_host, and one whose Origin is present and not allowed with 403
// forbidden_origin - and GRANTS CORS to everything else, which contract 2.12
// omits and the plan's adjudicated decision 1 adds. Without the grant the
// cockpit cannot make one REST call: every Tauri build is cross-origin against
// http://127.0.0.1:8730, so a GET fails at the response with no
// Access-Control-Allow-Origin header, and every JSON POST fails one step earlier
// at a preflight the mux does not route. WebSockets are exempt from all of it,
// which is exactly why Phase 1 worked with none of this.
func (p *OriginPolicy) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := p.Allow(origin)

		// Vary unconditionally. The response differs by Origin even when this
		// particular request carried none, and a cache that does not know that
		// will serve one origin's response to another.
		w.Header().Add("Vary", "Origin")
		if origin != "" && allowed {
			// Echoed verbatim, never "*", so the allowlist stays the single
			// decision point. No Access-Control-Allow-Credentials: no cookies
			// are used, and that header beside an echoed origin is what turns
			// an allowlist slip into an authenticated cross-origin call.
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		if !LoopbackHost(r.Host) {
			p.log.Warn("rejected a request whose Host is not loopback",
				"host", r.Host, "method", r.Method, "path", r.URL.Path,
				"remote", r.RemoteAddr)
			WriteProblem(w, http.StatusForbidden, CodeForbiddenHost,
				"This control plane only answers requests addressed to localhost.",
				map[string]string{"host": r.Host})
			return
		}

		if !allowed {
			// The allowlist goes in the log, not in the body: the body is read
			// by the page that was just rejected.
			p.log.Warn("rejected a request from an origin that is not allowed",
				"origin", origin, "allowed", strings.Join(p.order, ", "),
				"method", r.Method, "path", r.URL.Path)
			WriteProblem(w, http.StatusForbidden, CodeForbiddenOrigin,
				"This origin is not allowed to call the Luci App control plane. "+
					"The control plane log names the origins that are.",
				map[string]string{"origin": origin})
			return
		}

		// The origin != "" is a decision, not a missing case. An OPTIONS with no
		// Origin is not a preflight - a real preflight always carries one - so
		// answering it 204 here would bury a genuine routing question, "does
		// this path exist and what does it accept", inside a middleware that
		// knows nothing about routes. It falls through to the mux instead. An
		// OPTIONS that does carry an allowed origin is answered even without
		// Access-Control-Request-Method, because the response is identical
		// either way and the extra branch would buy nothing.
		if r.Method == http.MethodOptions && origin != "" {
			w.Header().Set("Access-Control-Allow-Methods", corsMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsHeaders)
			w.Header().Set("Access-Control-Max-Age", corsMaxAge)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
