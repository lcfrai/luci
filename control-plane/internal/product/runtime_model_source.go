package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Only a reviewed public HTTPS binary source. Private/metadata/loopback sources,
// archives, ambient credentials and arbitrary HTTP headers are not supported.
type RuntimeModelSource struct {
	ServiceID string `json:"serviceId"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
}
type RuntimeModelDownloadProof struct {
	Bytes      int64  `json:"bytes"`
	SHA256     string `json:"sha256"`
	ObservedAt string `json:"observedAt"`
}

func runtimeModelURL(raw string, redirect bool) (*url.URL, error) {
	if len(raw) > 8192 {
		return nil, ErrRuntimePreparation
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !redirect && (u.RawQuery != "" || len(raw) > 2048) {
		return nil, ErrRuntimePreparation
	}
	// A provider-generated public CDN signature may occur only in an ephemeral
	// redirect. The original URL is noncredential metadata; redirected URLs and
	// query tokens are never recorded or returned to model/UI/evidence.
	check := *u
	check.RawQuery = ""
	check.ForceQuery = false
	if _, err = researchURL(check.String()); err != nil {
		return nil, ErrRuntimePreparation
	}
	return u, nil
}
func validateRuntimeModelSource(s RuntimeModelSource) error {
	if !designIDPattern.MatchString(s.ServiceID) || !hexDigest(s.SHA256) || s.Bytes < 1 || s.Bytes > 8<<30 {
		return ErrRuntimePreparation
	}
	_, err := runtimeModelURL(s.URL, false)
	return err
}
func runtimeModelHTTPClient(ctx context.Context, authorize func(context.Context) error) *http.Client {
	transport := &http.Transport{Proxy: nil, DialContext: researchDial, DisableCompression: true, DisableKeepAlives: true, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 6 * time.Second, MaxResponseHeaderBytes: 32 << 10, ExpectContinueTimeout: time.Second}
	return &http.Client{Transport: transport, Timeout: 20 * time.Minute, CheckRedirect: func(request *http.Request, prior []*http.Request) error {
		if len(prior) > 3 {
			return ErrRuntimePreparation
		}
		if _, err := runtimeModelURL(request.URL.String(), true); err != nil {
			return err
		}
		request.Header.Del("Authorization")
		request.Header.Del("Cookie")
		request.Header.Del("Proxy-Authorization")
		return authorize(ctx)
	}}
}

type runtimeModelReader struct {
	mu        sync.Mutex
	ctx       context.Context
	body      io.ReadCloser
	expected  RuntimeModelSource
	count     int64
	hash      hash.Hash
	authorize func(context.Context) error
	record    func(context.Context, RuntimeModelDownloadProof) error
	verified  bool
	closed    bool
	failed    bool
}

func openRuntimeModel(ctx context.Context, source RuntimeModelSource, authorize func(context.Context) error, record func(context.Context, RuntimeModelDownloadProof) error, client *http.Client) (*runtimeModelReader, error) {
	if authorize == nil || record == nil || validateRuntimeModelSource(source) != nil {
		return nil, ErrRuntimePreparation
	}
	if err := authorize(ctx); err != nil {
		return nil, err
	}
	if client == nil {
		client = runtimeModelHTTPClient(ctx, authorize)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, ErrRuntimePreparation
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(request)
	if err != nil || response == nil || response.Body == nil {
		return nil, ErrRuntimePreparation
	}
	if response.StatusCode != http.StatusOK || response.ContentLength >= 0 && response.ContentLength != source.Bytes || response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		_ = response.Body.Close()
		return nil, ErrRuntimePreparation
	}
	if err = authorize(ctx); err != nil {
		_ = response.Body.Close()
		return nil, err
	}
	return &runtimeModelReader{ctx: ctx, body: response.Body, expected: source, hash: sha256.New(), authorize: authorize, record: record}, nil
}
func (r *runtimeModelReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.verified {
		return 0, io.EOF
	}
	if r.failed || r.closed || r.ctx.Err() != nil {
		return 0, ErrRuntimePreparation
	}
	if err := r.authorize(r.ctx); err != nil {
		r.failed = true
		return 0, err
	}
	if len(p) > 64<<10 {
		p = p[:64<<10]
	}
	remaining := r.expected.Bytes - r.count + 1
	if remaining < int64(len(p)) {
		p = p[:remaining]
	}
	n, err := r.body.Read(p)
	if n > 0 {
		r.count += int64(n)
		_, _ = r.hash.Write(p[:n])
	}
	if r.count > r.expected.Bytes {
		r.failed = true
		return n, ErrRuntimePreparation
	}
	if err != nil && !errors.Is(err, io.EOF) {
		r.failed = true
		return n, ErrRuntimePreparation
	}
	if errors.Is(err, io.EOF) {
		actual := hex.EncodeToString(r.hash.Sum(nil))
		if r.count != r.expected.Bytes || actual != r.expected.SHA256 {
			r.failed = true
			return n, ErrRuntimePreparation
		}
		closeErr := r.body.Close()
		r.closed = true
		if closeErr != nil {
			r.failed = true
			return n, ErrRuntimePreparation
		}
		if err = r.authorize(r.ctx); err != nil {
			r.failed = true
			return n, err
		}
		proof := RuntimeModelDownloadProof{Bytes: r.count, SHA256: actual, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		// Positive controller-side complete-body proof becomes durable BEFORE EOF is
		// passed to the one original upload. Failure aborts its body, never retries.
		if err = r.record(r.ctx, proof); err != nil {
			r.failed = true
			return n, err
		}
		r.verified = true
		return n, io.EOF
	}
	return n, nil
}
func (r *runtimeModelReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	return r.body.Close()
}
func runtimeModelSafeName(v string) bool { return runtimeArtifactPath(v) && !strings.HasPrefix(v, ".") }
