package e2bbe

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type providerSandbox struct {
	ID         string            `json:"sandboxID"`
	TemplateID string            `json:"templateID"`
	State      string            `json:"state"`
	CPU        int64             `json:"cpuCount"`
	MemoryMiB  int64             `json:"memoryMB"`
	Metadata   map[string]string `json:"metadata"`
}
type providerClient struct {
	endpoint *url.URL
	key      string
	http     *http.Client
}

func newClient(c Config) (*providerClient, error) {
	u, e := url.Parse(c.Endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Port() == "5008" {
		return nil, ErrUnsupported
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !privateIP(ip) {
		return nil, ErrUnsupported
	}
	if c.APIKey == "" || len(c.APIKey) > 4096 || strings.ContainsAny(c.APIKey, "\r\n\x00") {
		return nil, ErrUnsupported
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: c.RootCAs}
	transport.DialContext = privateDial
	transport.MaxConnsPerHost = 8
	transport.MaxIdleConnsPerHost = 4
	transport.ResponseHeaderTimeout = 10 * time.Second
	return &providerClient{endpoint: u, key: c.APIKey, http: &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("provider redirect refused") }}}, nil
}
func privateIP(ip net.IP) bool {
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback()) && !ip.IsUnspecified()
}
func privateDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, e
	}
	if port == "5008" {
		return nil, ErrUnsupported
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil {
		return nil, e
	}
	if len(ips) == 0 {
		return nil, ErrUnsupported
	}
	for _, ip := range ips {
		if !privateIP(ip.IP) {
			return nil, ErrUnsupported
		}
	}
	var last error
	for _, ip := range ips {
		conn, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if e == nil {
			return conn, nil
		}
		last = e
	}
	return nil, last
}
func (c *providerClient) call(ctx context.Context, method, path string, query url.Values, body any, out any) (int, http.Header, error) {
	var payload []byte
	if body != nil {
		var e error
		payload, e = json.Marshal(body)
		if e != nil {
			return 0, nil, e
		}
	}
	u := *c.endpoint
	u.Path = path
	u.RawPath = ""
	u.RawQuery = query.Encode()
	req, e := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(payload))
	if e != nil {
		return 0, nil, e
	}
	req.Header.Set("X-API-Key", c.key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// No transport retry/idempotency key: the durable controller sends Create
	// once only. Provider key expiry must never cause another allocation.
	response, e := c.http.Do(req)
	if e != nil {
		return 0, nil, ErrUnknown
	}
	defer response.Body.Close()
	data, e := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if e != nil || len(data) > maxResponse {
		return response.StatusCode, response.Header, ErrUnknown
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 && out != nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		if e = decoder.Decode(out); e != nil {
			return response.StatusCode, response.Header, ErrUnknown
		}
		var trailing any
		if decoder.Decode(&trailing) != io.EOF {
			return response.StatusCode, response.Header, ErrUnknown
		}
	}
	return response.StatusCode, response.Header, nil
}
func metadata(r Record) map[string]string {
	return map[string]string{"luci.owner": r.Request.Identity.OwnerID, "luci.project": r.Request.Identity.ProjectID, "luci.task": r.Request.Identity.TaskID, "luci.attempt": r.Request.Identity.AttemptID, "luci.operation": r.Request.Identity.OperationID, "luci.request": r.RequestDigest, "luci.scope": r.Deployment.Scope, "luci.generation": r.Deployment.HostGeneration}
}
func matches(r Record, s providerSandbox) bool {
	if !validID(s.ID) || s.TemplateID != r.Template.ID || s.MemoryMiB != r.Template.MemoryMiB || s.CPU*1000 != r.Template.CPUMillis || (s.State != "running" && s.State != "paused") {
		return false
	}
	for k, v := range metadata(r) {
		if s.Metadata[k] != v {
			return false
		}
	}
	return true
}
func (c *providerClient) create(ctx context.Context, r Record) (string, error) {
	body := map[string]any{"templateID": r.Template.ID, "timeout": r.Request.MaxSeconds, "autoPause": false, "autoResume": map[string]any{"enabled": false}, "network": map[string]any{"allowPublicTraffic": false, "denyOut": []string{"0.0.0.0/0"}}, "metadata": metadata(r)}
	var response providerSandbox
	status, _, e := c.call(ctx, http.MethodPost, "/v2/sandboxes", nil, body, &response)
	if e != nil || status != http.StatusCreated || !validID(response.ID) || response.TemplateID != r.Template.ID {
		return "", ErrUnknown
	}
	return response.ID, nil
}
func (c *providerClient) inspect(ctx context.Context, id string) (providerSandbox, bool, error) {
	var out providerSandbox
	status, _, e := c.call(ctx, http.MethodGet, "/sandboxes/"+url.PathEscape(id), nil, nil, &out)
	if e != nil {
		return out, false, e
	}
	if status == http.StatusNotFound {
		return out, true, nil
	}
	if status != http.StatusOK {
		return out, false, ErrUnknown
	}
	return out, false, nil
}
func (c *providerClient) find(ctx context.Context, r Record) (providerSandbox, error) {
	m := url.Values{}
	for k, v := range metadata(r) {
		m.Set(k, v)
	}
	query := url.Values{"metadata": []string{m.Encode()}, "limit": []string{strconv.Itoa(100)}, "state": []string{"running,paused"}}
	var rows []providerSandbox
	status, headers, e := c.call(ctx, http.MethodGet, "/v2/sandboxes", query, nil, &rows)
	if e != nil || status != http.StatusOK || headers.Get("X-Next-Token") != "" || len(rows) != 1 || !matches(r, rows[0]) {
		return providerSandbox{}, ErrUnknown
	}
	return rows[0], nil
}
func (c *providerClient) remove(ctx context.Context, id string) error {
	status, _, e := c.call(ctx, http.MethodDelete, "/sandboxes/"+url.PathEscape(id), nil, nil, nil)
	if e != nil || (status != http.StatusNoContent && status != http.StatusNotFound) {
		return ErrUnknown
	}
	return nil
}
