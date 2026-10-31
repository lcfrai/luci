package deploymentkube

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExistingTempo is private connection material supplied through an explicitly
// authorized setup. It is never an observation, response or model input.
type ExistingTempo struct {
	Endpoint string `json:"endpoint"`
	CAPEM    string `json:"caPem"`
	Token    string `json:"token,omitempty"`
	Tenant   string `json:"tenant,omitempty"`
}

type TraceQuery struct {
	OwnedBackend    bool   `json:"ownedBackend,omitempty"`
	ComponentID     string `json:"componentId"`
	PodUID          string `json:"podUid"`
	LookbackSeconds int64  `json:"lookbackSeconds"`
}

type TraceSummary struct {
	TraceID                 string `json:"traceId"`
	SpanCount               int    `json:"spanCount"`
	ErrorSpans              int    `json:"errorSpans"`
	FirstSpanAt             string `json:"firstSpanAt"`
	LastSpanAt              string `json:"lastSpanAt"`
	MatchingSpanWindowNanos string `json:"matchingSpanWindowNanos"`
}

type OperationalTraces struct {
	Storage        *StoredTelemetrySource `json:"storage,omitempty"`
	Identity       Identity               `json:"identity"`
	NamespaceUID   string                 `json:"namespaceUid"`
	Pod            OperationalPod         `json:"pod"`
	Query          TraceQuery             `json:"query"`
	StartAt        string                 `json:"startAt"`
	EndAt          string                 `json:"endAt"`
	ObservedAt     string                 `json:"observedAt"`
	State          string                 `json:"state"`
	Traces         []TraceSummary         `json:"traces"`
	Partial        bool                   `json:"partial"`
	EvidenceDigest string                 `json:"evidenceDigest"`
}

// The backend performs only the fixed query frontend GETs documented by Tempo.
// No arbitrary TraceQL/URL, logs, span names, attributes or messages are returned.
type TempoReader interface {
	Query(context.Context, string, string, string, time.Time, time.Time) ([]TraceSummary, bool, error)
}

type tempoHTTPReader struct {
	base       *url.URL
	connection ExistingTempo
	client     *http.Client
	authorize  func(context.Context) error
}

var tempoTenant = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
var tempoTraceID = regexp.MustCompile(`^(?:[a-f0-9]{16}|[a-f0-9]{32})$`)

func NewTempoReader(connection ExistingTempo, authorize func(context.Context) error) (TempoReader, error) {
	address, err := url.Parse(connection.Endpoint)
	if err != nil || address == nil || address.Scheme != "https" || address.Hostname() == "" || address.User != nil || address.RawQuery != "" || address.ForceQuery || address.Fragment != "" || address.RawPath != "" || len(connection.Endpoint) > 2048 || len(connection.CAPEM) < 1 || len(connection.CAPEM) > 256<<10 || len(connection.Token) > 16<<10 || strings.ContainsAny(connection.Token, "\r\n\x00") || connection.Tenant != "" && !tempoTenant.MatchString(connection.Tenant) || authorize == nil {
		return nil, ErrRefused
	}
	for _, part := range strings.Split(address.Path, "/") {
		if part == "." || part == ".." {
			return nil, ErrRefused
		}
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(connection.CAPEM)) {
		return nil, ErrRefused
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 0}).DialContext, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: address.Hostname()}, TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 32 << 10, DisableKeepAlives: true, ForceAttemptHTTP2: false}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &tempoHTTPReader{base: address, connection: connection, client: client, authorize: authorize}, nil
}

func (r *tempoHTTPReader) get(ctx context.Context, path string, query url.Values, value any) error {
	if r.authorize(ctx) != nil {
		return ErrRefused
	}
	address := *r.base
	address.Path = strings.TrimRight(address.Path, "/") + path
	address.RawQuery = query.Encode()
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(bounded, http.MethodGet, address.String(), nil)
	if err != nil {
		return ErrRefused
	}
	request.Header.Set("Accept", "application/json")
	if r.connection.Token != "" {
		request.Header.Set("Authorization", "Bearer "+r.connection.Token)
	}
	if r.connection.Tenant != "" {
		request.Header.Set("X-Scope-OrgID", r.connection.Tenant)
	}
	if r.authorize(bounded) != nil {
		return ErrRefused
	}
	response, err := r.client.Do(request)
	if err != nil {
		return ErrRefused
	}
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || len(raw) > 1<<20 || response.StatusCode != http.StatusOK || bounded.Err() != nil || r.authorize(ctx) != nil {
		return ErrRefused
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		return ErrRefused
	}
	return nil
}

type tempoAttribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue *string `json:"stringValue"`
	} `json:"value"`
}
type tempoSpan struct {
	TraceID string `json:"traceId"`
	SpanID  string `json:"spanId"`
	Start   string `json:"startTimeUnixNano"`
	End     string `json:"endTimeUnixNano"`
	Status  struct {
		Code json.RawMessage `json:"code"`
	} `json:"status"`
}
type tempoTrace struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []tempoAttribute `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []tempoSpan `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

func otlpHex(value string, bytes int) (string, bool) {
	nonzero := func(raw []byte) bool {
		for _, b := range raw {
			if b != 0 {
				return true
			}
		}
		return false
	}
	if len(value) == bytes*2 {
		raw, err := hex.DecodeString(value)
		if err == nil && len(raw) == bytes && nonzero(raw) {
			return hex.EncodeToString(raw), true
		}
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) != bytes || !nonzero(raw) {
		return "", false
	}
	return hex.EncodeToString(raw), true
}

func summarizeTempoTrace(value tempoTrace, traceID, namespace, service, podUID string, start, end time.Time) (TraceSummary, error) {
	result := TraceSummary{TraceID: traceID}
	var first, last uint64
	seen := map[string]bool{}
	totalSpans := 0
	if len(value.ResourceSpans) < 1 || len(value.ResourceSpans) > 64 {
		return result, ErrRefused
	}
	for _, resource := range value.ResourceSpans {
		if len(resource.Resource.Attributes) > 128 || len(resource.ScopeSpans) > 16 {
			return result, ErrRefused
		}
		attributes := map[string]string{}
		for _, attribute := range resource.Resource.Attributes {
			if attribute.Key != "service.namespace" && attribute.Key != "service.name" && attribute.Key != "service.instance.id" {
				continue
			}
			if _, exists := attributes[attribute.Key]; exists || attribute.Value.StringValue == nil {
				return result, ErrRefused
			}
			attributes[attribute.Key] = *attribute.Value.StringValue
		}
		matches := attributes["service.namespace"] == namespace && attributes["service.name"] == service && attributes["service.instance.id"] == podUID
		for _, scope := range resource.ScopeSpans {
			totalSpans += len(scope.Spans)
			if totalSpans > 256 {
				return result, ErrRefused
			}
			if !matches {
				continue
			}
			for _, span := range scope.Spans {
				actual, ok := otlpHex(span.TraceID, 16)
				if !ok || actual != traceID {
					return result, ErrRefused
				}
				spanID, ok := otlpHex(span.SpanID, 8)
				if !ok || seen[spanID] {
					return result, ErrRefused
				}
				seen[spanID] = true
				a, errA := strconv.ParseUint(span.Start, 10, 64)
				b, errB := strconv.ParseUint(span.End, 10, 64)
				if errA != nil || errB != nil || a < uint64(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()) || b < a || b > uint64(end.Add(time.Minute).UnixNano()) {
					return result, ErrRefused
				}
				code := strings.TrimSpace(string(span.Status.Code))
				isError := false
				switch code {
				case "", "null", "0", "1", `"STATUS_CODE_UNSET"`, `"STATUS_CODE_OK"`:
				case "2", `"STATUS_CODE_ERROR"`:
					isError = true
				default:
					return result, ErrRefused
				}
				// Tempo returns a whole trace, including valid parents/children outside
				// the search window. They are not malformed and do not broaden this view.
				// Retain only matching spans overlapping the requested time interval.
				if a > uint64(end.UnixNano()) || b < uint64(start.UnixNano()) {
					continue
				}
				if isError {
					result.ErrorSpans++
				}
				if result.SpanCount == 0 || a < first {
					first = a
				}
				if b > last {
					last = b
				}
				result.SpanCount++
			}
		}
	}
	if result.SpanCount == 0 {
		return result, nil
	}
	result.FirstSpanAt = time.Unix(0, int64(first)).UTC().Format(time.RFC3339Nano)
	result.LastSpanAt = time.Unix(0, int64(last)).UTC().Format(time.RFC3339Nano)
	result.MatchingSpanWindowNanos = strconv.FormatUint(last-first, 10)
	return result, nil
}

func (r *tempoHTTPReader) Query(ctx context.Context, namespace, service, podUID string, start, end time.Time) ([]TraceSummary, bool, error) {
	if namespace == "" || service == "" || podUID == "" || len(namespace) > 128 || len(service) > 63 || len(podUID) > 128 || end.Before(start) || end.Sub(start) > time.Hour {
		return nil, false, ErrRefused
	}
	// These three exact resource attributes are emitted by the existing SDK
	// artifact. Missing instrumentation produces no matching spans, never a wider
	// service-only query or a claim that the Collector stores/query traces.
	q := "{ resource.service.namespace = " + strconv.Quote(namespace) + " && resource.service.name = " + strconv.Quote(service) + " && resource.service.instance.id = " + strconv.Quote(podUID) + " }"
	params := url.Values{"q": {q}, "start": {strconv.FormatInt(start.Unix(), 10)}, "end": {strconv.FormatInt(end.Unix(), 10)}, "limit": {"4"}, "spss": {"1"}}
	var search struct {
		Traces []struct {
			TraceID string `json:"traceID"`
		} `json:"traces"`
	}
	if r.get(ctx, "/api/search", params, &search) != nil || len(search.Traces) > 4 {
		return nil, false, ErrRefused
	}
	// Search is sampled/first-N and trace retrieval can itself be partial. Even
	// fewer than four results never establishes exhaustive trace coverage.
	results := []TraceSummary{}
	seen := map[string]bool{}
	partial := true
	for _, candidate := range search.Traces {
		if !tempoTraceID.MatchString(candidate.TraceID) {
			return nil, false, ErrRefused
		}
		id := candidate.TraceID
		if len(id) == 16 {
			id = strings.Repeat("0", 16) + id
		}
		if seen[id] {
			return nil, false, ErrRefused
		}
		seen[id] = true
		var trace tempoTrace
		query := url.Values{"start": params["start"], "end": params["end"]}
		if r.get(ctx, "/api/traces/"+id, query, &trace) != nil {
			return nil, false, ErrRefused
		}
		summary, err := summarizeTempoTrace(trace, id, namespace, service, podUID, start, end)
		if err != nil {
			return nil, false, ErrRefused
		}
		if summary.SpanCount > 0 {
			results = append(results, summary)
		}
	}
	return results, partial, nil
}

func (c *Connector) ReadOperationalTraces(ctx context.Context, identity Identity, query TraceQuery, backend TempoReader) (OperationalTraces, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if backend == nil || query.LookbackSeconds < 1 || query.LookbackSeconds > 3600 || query.ComponentID == "" || query.PodUID == "" {
		return OperationalTraces{}, ErrRefused
	}
	operation, target, resources, err := c.operationsInput(ctx, identity)
	if err != nil {
		return OperationalTraces{}, ErrRefused
	}
	budget := 0
	before, err := c.currentOperationalPods(ctx, operation, target, resources, query.ComponentID, &budget)
	if err != nil {
		return OperationalTraces{}, ErrRefused
	}
	var selected *OperationalPod
	for i := range before.Pods {
		if before.Pods[i].UID == query.PodUID && before.Pods[i].ComponentID == query.ComponentID {
			selected = &before.Pods[i]
		}
	}
	if selected == nil || c.authorize(ctx, operation.Request, false) != nil {
		return OperationalTraces{}, ErrRefused
	}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(query.LookbackSeconds) * time.Second)
	traces, partial, err := backend.Query(ctx, identity.ProjectID, query.ComponentID, query.PodUID, start, end)
	if err != nil || len(traces) > 4 || c.authorize(ctx, operation.Request, false) != nil {
		return OperationalTraces{}, ErrRefused
	}
	latest, err := c.target(ctx, operation.Request)
	if err != nil || latest.NamespaceUID != operation.NamespaceUID {
		return OperationalTraces{}, ErrRefused
	}
	after, err := c.currentOperationalPods(ctx, operation, latest, resources, query.ComponentID, &budget)
	if err != nil {
		return OperationalTraces{}, ErrRefused
	}
	stable := false
	for _, pod := range after.Pods {
		if pod == *selected {
			stable = true
		}
	}
	if !stable || c.authorize(ctx, operation.Request, false) != nil || ctx.Err() != nil {
		return OperationalTraces{}, ErrRefused
	}
	state := "recorded"
	if partial {
		state = "partial"
	}
	if len(traces) == 0 {
		state = "no-data"
	}
	result := OperationalTraces{Identity: identity, NamespaceUID: operation.NamespaceUID, Pod: *selected, Query: query, StartAt: start.Format(time.RFC3339Nano), EndAt: end.Format(time.RFC3339Nano), ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), State: state, Traces: traces, Partial: partial}
	result.EvidenceDigest = digest(result)
	return result, nil
}
