package e2bbe

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type providerSandbox struct {
	ID                 string            `json:"sandboxID"`
	TemplateID         string            `json:"templateID"`
	State              string            `json:"state"`
	CPU                int64             `json:"cpuCount"`
	MemoryMiB          int64             `json:"memoryMB"`
	Metadata           map[string]string `json:"metadata"`
	EnvdAccessToken    string            `json:"envdAccessToken,omitempty"`
	TrafficAccessToken string            `json:"trafficAccessToken,omitempty"`
	EnvdVersion        string            `json:"envdVersion,omitempty"`
}

func (providerSandbox) String() string   { return "E2B sandbox response (private grant redacted)" }
func (providerSandbox) GoString() string { return "E2B sandbox response (private grant redacted)" }

type providerClient struct {
	endpoint *url.URL
	key      string
	adminKey string
	http     *http.Client
	egress   []string
}

func newClient(c Config) (*providerClient, error) {
	if !validToken(c.APIKey) {
		return nil, ErrUnsupported
	}
	u, h, e := privateHTTP(c.Endpoint, c.RootCAs, c.ClientCertificate)
	if e != nil {
		return nil, e
	}
	if c.AdminAPIKey != "" && (!validToken(c.AdminAPIKey) || c.AdminAPIKey == c.APIKey || !validID(c.HostNodeID)) {
		return nil, ErrUnsupported
	}
	return &providerClient{endpoint: u, key: c.APIKey, adminKey: c.AdminAPIKey, http: h, egress: append([]string{}, c.EgressAllow...)}, nil
}
func validToken(s string) bool {
	if len(s) < 1 || len(s) > 4096 {
		return false
	}
	for _, r := range s {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}
func privateHTTP(endpoint string, roots *x509.CertPool, clientCertificate *tls.Certificate) (*url.URL, *http.Client, error) {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Port() == "5008" {
		return nil, nil, ErrUnsupported
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !privateIP(ip) {
		return nil, nil, ErrUnsupported
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if clientCertificate != nil {
		if len(clientCertificate.Certificate) == 0 || clientCertificate.PrivateKey == nil {
			return nil, nil, ErrUnsupported
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{*clientCertificate}
	}
	transport.DialContext = privateDial
	transport.MaxConnsPerHost = 8
	transport.MaxIdleConnsPerHost = 4
	transport.ResponseHeaderTimeout = 10 * time.Second
	return u, &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("provider redirect refused") }}, nil
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
	return c.callWithKey(ctx, "X-API-Key", c.key, method, path, query, body, out)
}

func (c *providerClient) callWithKey(ctx context.Context, header, key, method, path string, query url.Values, body any, out any) (int, http.Header, error) {
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
	req.Header.Set(header, key)
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

type providerNode struct {
	ID                   string `json:"id"`
	Status               string `json:"status"`
	SandboxCount         *int64 `json:"sandboxCount"`
	SandboxStartingCount *int64 `json:"sandboxStartingCount"`
	MaxSandboxes         *int64 `json:"maxSandboxes"`
	Metrics              struct {
		AllocatedCPU         *int64 `json:"allocatedCPU"`
		CPUCount             *int64 `json:"cpuCount"`
		AllocatedMemoryBytes *int64 `json:"allocatedMemoryBytes"`
		MemoryUsedBytes      *int64 `json:"memoryUsedBytes"`
		MemoryTotalBytes     *int64 `json:"memoryTotalBytes"`
	} `json:"metrics"`
}

// VerifyNodeCapacity is a read-only, pinned-host advisory gate. The provider's
// atomic Create decision still owns actual placement; this cannot guarantee
// future quota, KVM availability or price. Unknown/malformed reads fail closed.
func (c *Controller) VerifyNodeCapacity(ctx context.Context) error {
	if c.config.DeploymentReadback != nil {
		if err := c.config.DeploymentReadback(ctx); err != nil {
			return ErrUnsupported
		}
	}
	if c.client.adminKey == "" || !validID(c.config.HostNodeID) {
		return ErrUnsupported
	}
	var nodes []providerNode
	status, _, err := c.client.callWithKey(ctx, "X-Admin-Token", c.client.adminKey, http.MethodGet, "/nodes", nil, nil, &nodes)
	if err != nil || status != http.StatusOK || len(nodes) == 0 || len(nodes) > 128 {
		return ErrUnsupported
	}
	var found *providerNode
	for i := range nodes {
		if !validID(nodes[i].ID) || nodes[i].ID != c.config.HostNodeID {
			continue
		}
		if found != nil {
			return ErrUnsupported
		}
		found = &nodes[i]
	}
	if found == nil || found.Status != "ready" || found.MaxSandboxes == nil || found.SandboxCount == nil || found.SandboxStartingCount == nil ||
		found.Metrics.CPUCount == nil || found.Metrics.AllocatedCPU == nil || found.Metrics.MemoryTotalBytes == nil || found.Metrics.AllocatedMemoryBytes == nil || found.Metrics.MemoryUsedBytes == nil {
		return ErrUnsupported
	}
	if *found.MaxSandboxes <= 0 || *found.SandboxCount < 0 || *found.SandboxStartingCount < 0 ||
		*found.Metrics.CPUCount <= 0 || *found.Metrics.AllocatedCPU < 0 || *found.Metrics.MemoryTotalBytes <= 0 || *found.Metrics.AllocatedMemoryBytes < 0 || *found.Metrics.MemoryUsedBytes < 0 {
		return ErrUnsupported
	}
	memory := c.config.Template.MemoryMiB * (1 << 20)
	cpu := c.config.Template.CPUMillis / 1000
	usedMemory := max(*found.Metrics.AllocatedMemoryBytes, *found.Metrics.MemoryUsedBytes)
	if *found.SandboxCount >= *found.MaxSandboxes || *found.SandboxStartingCount >= *found.MaxSandboxes-*found.SandboxCount ||
		*found.Metrics.AllocatedCPU > *found.Metrics.CPUCount-cpu || usedMemory > *found.Metrics.MemoryTotalBytes-memory {
		return ErrCapacity
	}
	return nil
}
func metadata(r Record) map[string]string {
	m := map[string]string{"luci.owner": r.Request.Identity.OwnerID, "luci.project": r.Request.Identity.ProjectID, "luci.task": r.Request.Identity.TaskID, "luci.attempt": r.Request.Identity.AttemptID, "luci.operation": r.Request.Identity.OperationID, "luci.request": r.RequestDigest, "luci.scope": r.Deployment.Scope, "luci.generation": r.Deployment.HostGeneration}
	if r.Deployment.Provider == "azure" {
		// One nonsecret digest ties readback to the exact configured Azure host,
		// disk and gateway without exposing their ARM IDs to the guest.
		m["luci.deployment"] = digest(r.Deployment)
	}
	if r.EgressDigest != "" {
		m["luci.egress"] = r.EgressDigest
	}
	if r.Template.BuildID != "" {
		m["luci.template-build"] = r.Template.BuildID
	}
	return m
}

// The pinned Embed API reports the exact build's status. This read is an
// admission prerequisite, not proof of guest contents or KVM isolation.
func (c *Controller) VerifyTemplateBuild(ctx context.Context) error {
	t := c.config.Template
	if t.BuildID == "" || !validID(t.BuildID) {
		return ErrUnsupported
	}
	var out struct {
		TemplateID string `json:"templateID"`
		BuildID    string `json:"buildID"`
		Status     string `json:"status"`
	}
	path := "/templates/" + t.ID + "/builds/" + t.BuildID + "/status"
	status, _, err := c.client.call(ctx, http.MethodGet, path, url.Values{"limit": {"0"}}, nil, &out)
	if err != nil || status != http.StatusOK || out.TemplateID != t.ID || out.BuildID != t.BuildID || out.Status != "ready" {
		return ErrUnsupported
	}
	// Create accepts only templateID. Its latest successful build can change
	// independently of this pinned build-status endpoint. Refuse a template
	// which already points elsewhere; exclusive operator publication is still
	// required because the read and Create are not atomic.
	var templates []struct {
		TemplateID string `json:"templateID"`
		BuildID    string `json:"buildID"`
		CPU        int64  `json:"cpuCount"`
		MemoryMiB  int64  `json:"memoryMB"`
		Public     *bool  `json:"public"`
	}
	status, _, err = c.client.call(ctx, http.MethodGet, "/templates", nil, nil, &templates)
	if err != nil || status != http.StatusOK || len(templates) == 0 || len(templates) > 128 {
		return ErrUnsupported
	}
	matches := 0
	for _, current := range templates {
		if current.TemplateID == t.ID {
			matches++
			if current.BuildID != t.BuildID || current.CPU*1000 != t.CPUMillis || current.MemoryMiB != t.MemoryMiB || current.Public == nil || *current.Public {
				return ErrUnsupported
			}
		}
	}
	if matches != 1 {
		return ErrUnsupported
	}
	return nil
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
func (c *providerClient) create(ctx context.Context, r Record) (providerSandbox, error) {
	body := map[string]any{"templateID": r.Template.ID, "timeout": r.Request.MaxSeconds, "autoPause": false, "autoResume": map[string]any{"enabled": false}, "network": map[string]any{"allowPublicTraffic": false, "allowOut": c.egress, "denyOut": []string{"0.0.0.0/0"}}, "metadata": metadata(r)}
	var response providerSandbox
	status, _, e := c.call(ctx, http.MethodPost, "/v2/sandboxes", nil, body, &response)
	if e != nil || status != http.StatusCreated || !validID(response.ID) || response.TemplateID != r.Template.ID {
		return providerSandbox{}, ErrUnknown
	}
	return response, nil
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
