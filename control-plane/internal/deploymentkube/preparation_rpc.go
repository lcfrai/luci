package deploymentkube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/client-go/rest"
)

// All payloads are server-constructed typed resource objects. There is no raw
// manifest, arbitrary URL, verb, exec, namespace or command browser surface.
var ErrPreparationAbsent = errors.New("the exact named preparation resource is absent")

type nonReplayablePreparationBody struct{ io.Reader }
type boundedPreparationBody struct {
	io.ReadCloser
	remaining int64
}

func (b *boundedPreparationBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		var probe [1]byte
		n, err := b.ReadCloser.Read(probe[:])
		if n > 0 || err == nil {
			return 0, ErrUnknown
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, err
}

type preparationTransport struct {
	base  http.RoundTripper
	limit int64
}

func (t preparationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil || response == nil {
		return nil, ErrUnknown
	}
	if response.Body == nil {
		return nil, ErrUnknown
	}
	// Never let client-go log/retain a provider error body that could echo private
	// Secret fields or the one-use upload frame. A failed RPC remains unknown.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		if request.Method == http.MethodGet && response.StatusCode == http.StatusNotFound {
			return nil, ErrPreparationAbsent
		}
		return nil, ErrUnknown
	}
	response.Header.Del("Warning")
	if response.ContentLength > t.limit {
		_ = response.Body.Close()
		return nil, ErrUnknown
	}
	response.Body = &boundedPreparationBody{ReadCloser: response.Body, remaining: t.limit}
	return response, nil
}
func preparationREST(target Target, timeout time.Duration, limit int64) (*rest.RESTClient, error) {
	if target.Client == nil || timeout <= 0 || timeout > 20*time.Minute || limit < 1 || limit > 1<<20 {
		return nil, ErrRefused
	}
	original, ok := target.Client.CoreV1().RESTClient().(*rest.RESTClient)
	if !ok || original == nil || original.Client == nil || original.Client.Transport == nil {
		return nil, ErrRefused
	}
	// Clone only this bounded request client; global kubeconfig, TLS, bearer
	// authentication, QPS/Burst and shared operation clients remain unchanged.
	client := *original.Client
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Transport = preparationTransport{base: original.Client.Transport, limit: limit}
	safe := *original
	safe.Client = &client
	return &safe, nil
}

// One fixed named namespaced resource RPC. Mutating callers must already have
// persisted the original attempted marker. No Retry-After or body recreation.
func PreparationResourceRPC(ctx context.Context, target Target, method, resource, name string, input, output any) error {
	if resource != "persistentvolumeclaims" && resource != "secrets" && resource != "pods" || method != http.MethodPost && method != http.MethodGet && method != http.MethodDelete || (method == http.MethodPost) != (name == "") {
		return ErrRefused
	}
	if name != "" && !token.MatchString(name) {
		return ErrRefused
	}
	client, err := preparationREST(target, 30*time.Second, 1<<20)
	if err != nil {
		return err
	}
	request := client.Verb(method).Namespace(target.Namespace).Resource(resource).MaxRetries(0).WarningHandler(rest.NoWarnings{})
	if name != "" {
		request = request.Name(name)
	}
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil || len(raw) > 512<<10 {
			return ErrRefused
		}
		request = request.SetHeader("Content-Type", "application/json").Body(nonReplayablePreparationBody{Reader: bytes.NewReader(raw)})
	}
	stream, err := request.Stream(ctx)
	if err != nil {
		if errors.Is(err, ErrPreparationAbsent) {
			return ErrPreparationAbsent
		}
		return ErrUnknown
	}
	data, readErr := io.ReadAll(io.LimitReader(stream, (1<<20)+1))
	closeErr := stream.Close()
	if readErr != nil || closeErr != nil || len(data) > 1<<20 || ctx.Err() != nil {
		return ErrUnknown
	}
	if output != nil && json.Unmarshal(data, output) != nil {
		return ErrUnknown
	}
	return nil
}

type ModelSinkReceipt struct {
	Protocol      int    `json:"protocol"`
	PreparationID string `json:"preparationId"`
	OperationID   string `json:"operationId"`
	PVCUID        string `json:"pvcUid"`
	Path          string `json:"path"`
	Bytes         int64  `json:"bytes"`
	SHA256        string `json:"sha256"`
	State         string `json:"state"`
}

func DecodeModelSinkReceipt(raw []byte) (ModelSinkReceipt, error) {
	var out ModelSinkReceipt
	if len(raw) > 2048 {
		return out, ErrUnknown
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return out, ErrUnknown
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok {
			return out, ErrUnknown
		}
		if _, duplicate := fields[name]; duplicate {
			return out, ErrUnknown
		}
		switch name {
		case "protocol", "preparationId", "operationId", "pvcUid", "path", "bytes", "sha256", "state":
		default:
			return out, ErrUnknown
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return out, ErrUnknown
		}
		fields[name] = value
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') || len(fields) != 8 || !errors.Is(decoder.Decode(new(any)), io.EOF) {
		return out, ErrUnknown
	}
	normalized, err := json.Marshal(fields)
	if err != nil || json.Unmarshal(normalized, &out) != nil || out.Protocol != 1 || !token.MatchString(out.PreparationID) || !token.MatchString(out.OperationID) || !token.MatchString(out.PVCUID) || out.Bytes < 1 || out.Bytes > 8<<30 || !hashPattern.MatchString(out.SHA256) || out.State != "stored-reported" || out.Path == "" || len(out.Path) > 240 {
		return ModelSinkReceipt{}, ErrUnknown
	}
	return out, nil
}

func UploadOriginalModel(ctx context.Context, target Target, podName, key string, payload io.Reader, before func(context.Context) error) (ModelSinkReceipt, error) {
	if !token.MatchString(podName) || len(podName) > 63 || !hashPattern.MatchString(key) || payload == nil || before == nil {
		return ModelSinkReceipt{}, ErrRefused
	}
	client, err := preparationREST(target, 20*time.Minute, 8192)
	if err != nil {
		return ModelSinkReceipt{}, err
	}
	if err = before(ctx); err != nil {
		return ModelSinkReceipt{}, err
	}
	// Fixed path/port only. The secret is body framing, not a URL/header/CLI or a
	// replayable byte slice. client-go sees io.Reader, never logs bodyBytes.
	body := nonReplayablePreparationBody{Reader: io.MultiReader(strings.NewReader(key+"\n"), payload)}
	request := client.Post().Namespace(target.Namespace).Resource("pods").Name(podName+":9090").SubResource("proxy").Suffix("upload").MaxRetries(0).WarningHandler(rest.NoWarnings{}).SetHeader("Content-Type", "application/octet-stream").Body(body)
	stream, err := request.Stream(ctx)
	if err != nil {
		return ModelSinkReceipt{}, ErrUnknown
	}
	raw, readErr := io.ReadAll(io.LimitReader(stream, 2049))
	closeErr := stream.Close()
	if readErr != nil || closeErr != nil || len(raw) > 2048 || ctx.Err() != nil {
		return ModelSinkReceipt{}, ErrUnknown
	}
	receipt, err := DecodeModelSinkReceipt(raw)
	if err != nil {
		return ModelSinkReceipt{}, err
	}
	if err = before(ctx); err != nil {
		return ModelSinkReceipt{}, err
	}
	return receipt, nil
}
