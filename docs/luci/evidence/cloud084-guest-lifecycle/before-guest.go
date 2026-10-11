package e2bbe

import (
	"context"
	"crypto/x509"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const guestInputPath = "/luci-transfer/input.tar"
const guestOutputPath = "/luci-transfer/output.tar"

// GuestConfig is a private transport grant, never a public API response or a
// context artifact. Callers must bind the origins/tokens to this exact Create
// response and trusted template. Lost grants cannot invoke Connect implicitly:
// that upstream operation may resume a VM and extend its paid lifetime.
type GuestConfig struct {
	Record             Record
	EnvdEndpoint       string
	SandboxEndpoint    string
	EnvdVersion        string
	EnvdAccessToken    string                                        `json:"-"`
	TrafficAccessToken string                                        `json:"-"`
	RootCAs            *x509.CertPool                                `json:"-"`
	CurrentRecord      func(context.Context, string) (Record, error) `json:"-"`
	Authorize          func(context.Context, Request, string) error  `json:"-"`
}

func (GuestConfig) String() string { return "E2B guest transport configuration (credentials redacted)" }
func (GuestConfig) GoString() string {
	return "E2B guest transport configuration (credentials redacted)"
}

type GuestTransport struct {
	owner         *Controller
	mu            sync.Mutex
	closed        bool
	config        GuestConfig
	envd, sandbox *url.URL
	files, attach *http.Client
}

func NewGuestTransport(cfg GuestConfig) (*GuestTransport, error) {
	if !validRequest(cfg.Record.Request) || cfg.Record.RequestDigest != digest(cfg.Record.Request) || !validID(cfg.Record.SandboxID) || cfg.Record.State != Allocated || !cfg.Record.ResourcesHeld || cfg.Record.CancelRequested || cfg.Record.CreatedAt.IsZero() || cfg.EnvdVersion != EnvdVersion || !validToken(cfg.EnvdAccessToken) || !validToken(cfg.TrafficAccessToken) || cfg.CurrentRecord == nil || cfg.Authorize == nil {
		return nil, ErrUnsupported
	}
	envd, files, e := privateHTTP(cfg.EnvdEndpoint, cfg.RootCAs)
	if e != nil {
		return nil, e
	}
	sandbox, attach, e := privateHTTP(cfg.SandboxEndpoint, cfg.RootCAs)
	if e != nil {
		files.CloseIdleConnections()
		return nil, e
	}
	// A WebSocket outlives the bounded upgrade request. Its reads/writes use the
	// existing sandboxd frame/context protocol, not http.Client.Timeout.
	attach.Timeout = 0
	return &GuestTransport{config: cfg, envd: envd, sandbox: sandbox, files: files, attach: attach}, nil
}
func (g *GuestTransport) guard(ctx context.Context, action string) error {
	g.mu.Lock()
	closed := g.closed
	g.mu.Unlock()
	if closed {
		return ErrClosed
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	r := g.config.Record
	if !time.Now().Before(r.CreatedAt.Add(time.Duration(r.Request.MaxSeconds) * time.Second)) {
		return ErrUnknown
	}
	check := func() error {
		now, e := g.config.CurrentRecord(ctx, r.Request.Identity.OperationID)
		if e != nil {
			return e
		}
		if now.SandboxID != r.SandboxID || now.RequestDigest != r.RequestDigest || now.RequestDigest != digest(now.Request) || now.Template != r.Template || !now.CreatedAt.Equal(r.CreatedAt) || now.Deployment != r.Deployment || now.State != Allocated || now.CancelRequested || !now.ResourcesHeld {
			return ErrConflict
		}
		return nil
	}
	if e := check(); e != nil {
		return e
	}
	if e := g.config.Authorize(ctx, r.Request, action); e != nil {
		return e
	}
	return check()
}
func (g *GuestTransport) fileRequest(ctx context.Context, method, path string, body io.Reader, length int64) (*http.Response, error) {
	u := *g.envd
	u.Path = "/files"
	u.RawQuery = url.Values{"path": {path}, "username": {"root"}}.Encode()
	req, e := http.NewRequestWithContext(ctx, method, u.String(), body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("X-Access-Token", g.config.EnvdAccessToken)
	req.Header.Set("e2b-traffic-access-token", g.config.TrafficAccessToken)
	if body != nil {
		req.ContentLength = length
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	response, e := g.files.Do(req)
	if e != nil {
		return nil, ErrUnknown
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		if response.StatusCode == 401 || response.StatusCode == 403 {
			return nil, backend.ErrUnauthorized
		}
		return nil, ErrUnknown
	}
	return response, nil
}

type countReader struct {
	r io.Reader
	n int64
}

func (r *countReader) Read(b []byte) (int, error) { n, e := r.r.Read(b); r.n += int64(n); return n, e }

// StageInput validates before any upload, writes one fixed root-owned archive
// path and verifies the complete remote readback. It does not extract, launch a
// command or grant authority. The trusted guest bootstrap is a separate gate.
func (g *GuestTransport) StageInput(ctx context.Context, r io.ReadSeeker) (TransferManifest, error) {
	var m TransferManifest
	if r == nil {
		return m, ErrTransfer
	}
	if e := g.guard(ctx, "stage-input"); e != nil {
		return m, e
	}
	if _, e := r.Seek(0, io.SeekStart); e != nil {
		return m, e
	}
	req := g.config.Record.Request
	binding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "input"}
	counter := &countReader{r: r}
	m, e := VerifyTransfer(contextReader{ctx, counter}, binding, req.BundleDigest)
	if e != nil {
		return m, e
	}
	if _, e = r.Seek(0, io.SeekStart); e != nil {
		return m, e
	}
	if e = g.guard(ctx, "stage-input"); e != nil {
		return m, e
	}
	response, e := g.fileRequest(ctx, http.MethodPost, guestInputPath, io.LimitReader(contextReader{ctx, r}, counter.n), counter.n)
	if e != nil {
		return m, e
	}
	drained, drainErr := io.Copy(io.Discard, io.LimitReader(response.Body, 4097))
	response.Body.Close()
	if drainErr != nil || drained > 4096 {
		return m, ErrUnknown
	}
	if e = g.guard(ctx, "stage-input"); e != nil {
		return m, e
	}
	return g.verifyInput(ctx)
}

func (g *GuestTransport) verifyInput(ctx context.Context) (TransferManifest, error) {
	var m TransferManifest
	if e := g.guard(ctx, "stage-input"); e != nil {
		return m, e
	}
	response, e := g.fileRequest(ctx, http.MethodGet, guestInputPath, nil, 0)
	if e != nil {
		return m, e
	}
	defer response.Body.Close()
	req := g.config.Record.Request
	binding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "input"}
	got, e := VerifyTransfer(contextReader{ctx, response.Body}, binding, req.BundleDigest)
	if e != nil {
		return m, e
	}
	if e = g.guard(ctx, "stage-input"); e != nil {
		return m, e
	}
	return got, nil
}

// ReceiveOutput verifies bytes into the bounded durable store. Digest equality
// proves transport integrity only, not that a task/check succeeded or source is
// approved for integration. No original project files are modified here.
func (g *GuestTransport) ReceiveOutput(ctx context.Context, hash string, store *DirectoryStore) (TransferManifest, error) {
	var m TransferManifest
	if store == nil || !digestPattern.MatchString(hash) {
		return m, ErrUnsupported
	}
	if e := g.guard(ctx, "collect-output"); e != nil {
		return m, e
	}
	response, e := g.fileRequest(ctx, http.MethodGet, guestOutputPath, nil, 0)
	if e != nil {
		return m, e
	}
	defer response.Body.Close()
	req := g.config.Record.Request
	binding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "output"}
	m, e = store.PutTransfer(ctx, binding, hash, response.Body)
	if e != nil {
		return m, e
	}
	if e = g.guard(ctx, "collect-output"); e != nil {
		return m, e
	}
	return m, nil
}
func (g *GuestTransport) Attach(ctx context.Context, token string) (*websocket.Conn, error) {
	if !validToken(token) {
		return nil, backend.ErrUnauthorized
	}
	if e := g.guard(ctx, "attach"); e != nil {
		return nil, e
	}
	u := *g.sandbox
	u.Scheme = "wss"
	u.Path = backend.AttachPath
	ctx, cancel := context.WithTimeout(ctx, backend.DialTimeout)
	defer cancel()
	headers := http.Header{"Authorization": {"Bearer " + token}, "E2b-Traffic-Access-Token": {g.config.TrafficAccessToken}}
	conn, response, e := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPClient: g.attach, HTTPHeader: headers})
	if e != nil {
		if response != nil && (response.StatusCode == 401 || response.StatusCode == 403) {
			return nil, backend.ErrUnauthorized
		}
		return nil, ErrUnknown
	}
	conn.SetReadLimit(backend.MaxMessageBytes)
	if e = g.guard(ctx, "attach"); e != nil {
		conn.CloseNow()
		return nil, e
	}
	return conn, nil
}
func (g *GuestTransport) Close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	g.files.CloseIdleConnections()
	g.attach.CloseIdleConnections()
}
