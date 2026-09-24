// Package panacea implements deploy.Deployer over Panacea's panacea-api (OD-13: phase 7 is a
// client of its existing API). It is the ONLY package that knows a Panacea route, envelope or
// field spelling. Every DTO in wire.go names the Panacea file and line it mirrors.
//
// This file is the transport (task 02): the one request path, do, that tasks 03 and 04 build
// their methods on. Nothing else in the repository opens a connection to Panacea.
package panacea

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// TokenSource yields the signed-in user's own pnc_ token. connection.Manager (task 05)
// implements it over the credential store. deploy.ErrNotConnected when none is stored.
type TokenSource interface {
	PanaceaToken(ctx context.Context) (string, error)
}

type Config struct {
	// Origin is scheme://host[:port], no path — ParseOrigin's output. Every Panacea route
	// already begins /api/v1/ (routes.go), so the client appends paths verbatim. New runs
	// ParseOrigin on it and keeps the canonical form.
	Origin string
	Tokens TokenSource
	// CAFile, when set, is a PEM bundle ADDED to the system roots. Empty uses the platform
	// verifier alone. There is no option to skip verification.
	CAFile string
	// Timeout bounds each attempt. Zero means 15s: POST …/deploy itself waits up to 15 s inside
	// Panacea (handlers/projects.go:2985), so a lower value would abandon deploys that succeed.
	Timeout time.Duration
	// HTTPClient replaces the built client. Tests only. New copies it and forces the two rules
	// every Panacea call keeps whatever client carries it: redirects are never followed, and no
	// cookie jar is attached (the bearer is the only credential sent).
	HTTPClient *http.Client
	Log        *slog.Logger // nil = slog.Default()
	UserAgent  string       // "PanaceaApp/<version>"; "" = "PanaceaApp"
}

const (
	// DefaultTimeout is Config.Timeout's zero value (see Config.Timeout for why 15 s).
	DefaultTimeout = 15 * time.Second
	// maxBody is the most of any answer do reads: a 2xx over it is unusable, never truncated.
	maxBody = 8 << 20
	// maxInlineRetryAfter is the longest Retry-After do waits out itself (review M3; b07 round
	// 2, M-3). A longer one is returned on Error.RetryAfter, and no caller waits on it yet (S7-R1).
	maxInlineRetryAfter = 2 * time.Second
	apiPrefix           = "/api/v1/"
)

// retryDelays are the waits before a GET's second and third attempts. POST, PUT and DELETE
// are never retried: a retried deploy is a second deploy.
var retryDelays = [...]time.Duration{250 * time.Millisecond, time.Second}

type Client struct {
	cfg     Config // Origin canonical, UserAgent defaulted
	http    *http.Client
	log     *slog.Logger
	timeout time.Duration
	// sleep waits between GET attempts. It returns early with ctx's error when ctx ends.
	// Tests replace it to record the waits (review M3).
	sleep func(ctx context.Context, d time.Duration) error
}

// ErrSignInRedirect: the address answered with a redirect, which Panacea's API never does and
// its SSO proxy always does. Its sentence names the likely fix. Version returns a copy that
// carries the status received and wraps this value, so errors.Is(err, ErrSignInRedirect) holds.
var ErrSignInRedirect = &deploy.Error{
	Kind:    deploy.KindRejected,
	Op:      "version",
	Message: "That address sends you to a sign-in page, so it is the portal, not the API. Try https://dev.panacea.kpmg.",
}

// notTheAPI is Version's sentence for a 2xx that is not {"version":"<non-empty>"}.
const notTheAPI = "That address answered, but not as the Panacea API."

// New validates cfg and builds the client. Errors are sentences a person can act on.
func New(cfg Config) (*Client, error) {
	origin, err := ParseOrigin(cfg.Origin)
	if err != nil {
		return nil, err
	}
	cfg.Origin = origin
	if cfg.Tokens == nil {
		return nil, errors.New("the Panacea connector was built with no credential source; sign in to Panacea first")
	}
	if cfg.Timeout < 0 {
		return nil, fmt.Errorf("the Panacea timeout must be positive, not %v", cfg.Timeout)
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	var hc *http.Client
	if cfg.HTTPClient != nil {
		// A test's client would silently drop the CA file: refuse the pair instead.
		if cfg.CAFile != "" {
			return nil, errors.New("panacea.Config: HTTPClient and CAFile cannot both be set")
		}
		cp := *cfg.HTTPClient
		cp.CheckRedirect = neverFollow
		cp.Jar = nil
		hc = &cp
	} else if hc, err = BuildHTTPClient(cfg.CAFile, timeout); err != nil {
		return nil, err
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "PanaceaApp"
	}
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	return &Client{cfg: cfg, http: hc, log: log, timeout: timeout, sleep: sleepCtx}, nil
}

// The four refusals of ParseOrigin, each naming its fix.
const (
	originNoScheme = "The Panacea address must start with https://."
	originHasPath  = "Give the address without a path: https://dev.panacea.kpmg, not https://dev.panacea.kpmg/api."
	originUserinfo = "The address must not contain a user name or password."
	originNoHost   = "That is not a web address."
)

// refuseOrigin carries no Cause: url.Parse's error quotes the input, which may hold a password.
func refuseOrigin(sentence string) error {
	return &deploy.Error{Kind: deploy.KindRejected, Op: "origin", Message: sentence}
}

// ParseOrigin returns the canonical origin ("https://dev.panacea.kpmg", lowercased host, no
// trailing slash, default port dropped). A refusal is a *deploy.Error, KindRejected, whose
// Message is one of the four sentences above.
func ParseOrigin(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", refuseOrigin(originNoHost)
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", refuseOrigin(originNoHost)
	}
	switch {
	case u.Scheme != "https": // url.Parse lowercases the scheme
		return "", refuseOrigin(originNoScheme)
	case u.User != nil:
		return "", refuseOrigin(originUserinfo)
	case u.Opaque != "" || u.Hostname() == "":
		return "", refuseOrigin(originNoHost)
	case (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "":
		return "", refuseOrigin(originHasPath)
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if port == "443" {
		port = ""
	}
	if port != "" {
		return "https://" + net.JoinHostPort(host, port), nil
	}
	if strings.Contains(host, ":") { // IPv6
		return "https://[" + host + "]", nil
	}
	return "https://" + host, nil
}

// neverFollow is every Panacea client's CheckRedirect: Panacea's API never redirects, and its
// SSO proxy always does, so a redirect is an answer to report, not a hop to take.
func neverFollow(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// BuildHTTPClient is the one place TLS is configured:
//
//	the transport is http.DefaultTransport.(*http.Transport).Clone(), never a fresh
//	&http.Transport{} (whose Proxy is nil). The clone keeps Proxy: http.ProxyFromEnvironment
//	(HTTPS_PROXY, NO_PROXY), ForceAttemptHTTP2 (which keeps HTTP/2 once TLSClientConfig is
//	set), and the idle-connection and handshake timeouts. Only TLSClientConfig is replaced;
//	RootCAs nil when caFile is "" (the platform verifier; on Windows, the Windows store);
//	otherwise x509.SystemCertPool() plus caFile's certificates;
//	MinVersion TLS 1.2; CheckRedirect returns http.ErrUseLastResponse (never follow).
//	Go reads the proxy from the environment only, never the Windows system or PAC proxy.
//	Whether the owner's network needs one for Panacea is unverified: LIVE-CHECK.md step 1
//	answers it, and the fix is HTTPS_PROXY, not code (review M2).
//
// timeout bounds each request; zero or less is DefaultTimeout. A caFile that cannot be read,
// or that holds no PEM certificate, is an error naming the file: it is never ignored.
func BuildHTTPClient(caFile string, timeout time.Duration) (*http.Client, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http.DefaultTransport is a %T, not an *http.Transport; something in this process replaced it", http.DefaultTransport)
	}
	tr := base.Clone()
	tc := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		pemBytes, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("the Panacea CA file %s cannot be read: %w", caFile, err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("the system certificate store cannot be read, so %s cannot be added to it: %w", caFile, err)
		}
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("the Panacea CA file %s holds no PEM certificate", caFile)
		}
		tc.RootCAs = pool
	}
	tr.TLSClientConfig = tc
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: neverFollow}, nil
}

// sleepCtx is Client.sleep's production value: it waits d, or returns ctx's error first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Version calls GET <origin>/api/v1/version, unauthenticated (routes.go:630), and returns the
// "version" string. It exists before any Client does: the connect form proves an address is a
// Panacea API with it before accepting it. A redirect is NOT followed, whatever hc's own
// CheckRedirect says, and is reported as ErrSignInRedirect (errors.Is) — the api.dev host 307s
// to /oauth2/sign_in (orchestrator-rulings.md). It makes one attempt and never retries: the
// connect form shows its answer at once. hc nil is BuildHTTPClient("", DefaultTimeout).
//
// origin goes through ParseOrigin first; its refusal is returned before any request. A 2xx that
// is not {"version":"<non-empty string>"} is KindRejected with notTheAPI's sentence; any other
// non-2xx follows the status table (errors.go), with Op "version".
func Version(ctx context.Context, hc *http.Client, origin string) (string, error) {
	o, err := ParseOrigin(origin)
	if err != nil {
		return "", err
	}
	if hc == nil {
		if hc, err = BuildHTTPClient("", DefaultTimeout); err != nil {
			return "", err
		}
	}
	cp := *hc
	cp.CheckRedirect = neverFollow
	cp.Jar = nil
	const path = apiPrefix + "version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o+path, nil)
	if err != nil {
		return "", &deploy.Error{Kind: deploy.KindRejected, Op: "version", Cause: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "PanaceaApp")
	req.Header.Set("X-Request-Id", ids.UUID())
	resp, err := cp.Do(req)
	if err != nil {
		return "", &deploy.Error{Kind: deploy.KindUnavailable, Op: "version", Cause: err}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	status := resp.StatusCode
	switch {
	case status >= 300 && status < 400:
		e := *ErrSignInRedirect
		e.Status = status
		e.Cause = fmt.Errorf("GET %s answered %d, Location %q: %w", path, status, resp.Header.Get("Location"), ErrSignInRedirect)
		return "", &e
	case status < 200 || status >= 300:
		e := mapStatus("version", status, readProblem(body), parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()))
		e.Cause = fmt.Errorf("GET %s answered %d", path, status)
		return "", e
	case readErr != nil:
		return "", &deploy.Error{Kind: deploy.KindUnavailable, Op: "version", Status: status, Cause: readErr}
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.Version == "" || len(body) > maxBody {
		return "", &deploy.Error{Kind: deploy.KindRejected, Op: "version", Message: notTheAPI, Status: status,
			Cause: fmt.Errorf("GET %s answered %d with %d bytes that are not {\"version\":\"…\"}", path, status, len(body))}
	}
	return v.Version, nil
}

// do is the one request path. Unexported; tasks 03 and 04 call it.
//
//   - Authorization: Bearer <token from Tokens>; Accept: application/json; User-Agent;
//     X-Request-Id: ids.UUID(), one per call (Panacea echoes it as X-Client-Request-Id,
//     middleware/requestid.go:28-32). No cookie is ever sent. No token, or deploy.ErrNotConnected
//     from Tokens, returns before any request.
//   - path must begin /api/v1/; every segment a caller took from input is url.PathEscape'd by
//     that caller, and do sends it as given (a branch name contains /).
//   - GET only: a transport failure, 502, 503 or 504 is retried twice, after 250 ms and 1 s.
//     POST, PUT and DELETE are NEVER retried: a retried deploy is a second deploy. A 503's
//     Retry-After of at most 2 s makes that wait max(the fixed delay, Retry-After); a longer one
//     ends the retries at once and is returned on Error.RetryAfter (S7-R1). A 429 is not retried.
//   - Timeout bounds each attempt, not the loop.
//   - A 2xx body is read up to 8 MiB into out (json.Unmarshal into a fresh value, set on out only
//     when it decodes). An unreadable 2xx body, or a 2xx outside okStatus when okStatus is given,
//     is KindUnavailable with Op and that Status, never a zero value handed back as success.
//     out nil discards the body.
//   - A non-2xx is readProblem + mapStatus (errors.go). The body is redacted of the token first.
//   - One Warn line per failed call: op, method, path, status, duration, attempts, Panacea's
//     X-Request-Id. Never the Authorization header, never a request body. The redacted error
//     body goes to Debug, at most 64 KiB of it.
func (c *Client) do(ctx context.Context, op, method, path string, in, out any, okStatus ...int) error {
	if !strings.HasPrefix(path, apiPrefix) {
		return &deploy.Error{Kind: deploy.KindRejected, Op: op,
			Cause: fmt.Errorf("panacea: path %q does not begin %s", path, apiPrefix)}
	}
	token, err := c.cfg.Tokens.PanaceaToken(ctx)
	if err != nil {
		var de *deploy.Error
		if errors.As(err, &de) {
			return err
		}
		return &deploy.Error{Kind: deploy.KindNotConnected, Op: op, Cause: err}
	}
	if token == "" {
		return deploy.ErrNotConnected
	}
	var payload []byte
	if in != nil {
		if payload, err = json.Marshal(in); err != nil {
			return &deploy.Error{Kind: deploy.KindRejected, Op: op, Cause: fmt.Errorf("panacea: encoding the %s body: %w", op, err)}
		}
	}
	attempts := 1
	if method == http.MethodGet {
		attempts += len(retryDelays)
	}
	rid, start := ids.UUID(), time.Now()
	var res outcome
	n := 1
	for ; ; n++ {
		res = c.attempt(ctx, op, method, path, token, rid, payload, out, okStatus)
		if res.err == nil {
			return nil
		}
		if n >= attempts || !res.retryable || ctx.Err() != nil || res.err.RetryAfter > maxInlineRetryAfter {
			break
		}
		delay := max(retryDelays[n-1], res.err.RetryAfter)
		c.log.Debug("panacea attempt failed; retrying", "op", op, "method", method, "path", path,
			"attempt", n, "status", res.err.Status, "delay", delay)
		if c.sleep(ctx, delay) != nil {
			break
		}
	}
	c.log.Warn("panacea call failed", "op", op, "method", method, "path", path, "status", res.err.Status,
		"kind", string(res.err.Kind), "duration", time.Since(start), "attempts", n,
		"panacea_request_id", res.requestID, "cause", causeText(res.err))
	if len(res.body) > 0 {
		c.log.Debug("panacea error body", "op", op, "status", res.err.Status, "body", string(res.body[:min(len(res.body), 64<<10)]))
	}
	return res.err
}

// outcome is one attempt's result.
type outcome struct {
	err       *deploy.Error // nil on success
	retryable bool          // a transport failure (including a body cut short), 502, 503 or 504
	requestID string        // Panacea's X-Request-Id, when it answered
	body      []byte        // a non-2xx body, redacted of the token, for the Debug line
}

func (c *Client) attempt(ctx context.Context, op, method, path, token, rid string, payload []byte, out any, okStatus []int) outcome {
	actx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(actx, method, c.cfg.Origin+path, body)
	if err != nil {
		return outcome{err: &deploy.Error{Kind: deploy.KindRejected, Op: op, Cause: err}}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("X-Request-Id", rid)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return outcome{err: &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Cause: err}, retryable: true}
	}
	defer resp.Body.Close()
	o := outcome{requestID: resp.Header.Get("X-Request-Id")}
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	status := resp.StatusCode
	if status < 200 || status >= 300 {
		o.body = redact(raw, token)
		o.err = mapStatus(op, status, readProblem(o.body), parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()))
		o.err.Cause = fmt.Errorf("panacea: %s %s answered %d", method, path, status)
		o.retryable = status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
		return o
	}
	unusable := func(cause error) outcome {
		o.err = &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: status,
			Cause: fmt.Errorf("panacea: %s %s answered %d: %w", method, path, status, cause)}
		return o
	}
	if len(okStatus) > 0 && !slices.Contains(okStatus, status) {
		return unusable(fmt.Errorf("this call accepts only %v", okStatus))
	}
	if out == nil {
		return o
	}
	if readErr != nil {
		o.retryable = true // the body was cut short: a transport failure
		return unusable(readErr)
	}
	if len(raw) > maxBody {
		return unusable(fmt.Errorf("the body is over %d bytes", maxBody))
	}
	dst := reflect.ValueOf(out)
	if dst.Kind() != reflect.Pointer || dst.IsNil() {
		return unusable(fmt.Errorf("out is a %T, not a non-nil pointer", out))
	}
	fresh := reflect.New(dst.Elem().Type())
	if err := json.Unmarshal(raw, fresh.Interface()); err != nil {
		return unusable(err)
	}
	dst.Elem().Set(fresh.Elem())
	return o
}

// causeText is the Warn line's cause: Cause's text, which never holds a body or a header.
func causeText(e *deploy.Error) string {
	if e.Cause == nil {
		return ""
	}
	return e.Cause.Error()
}
