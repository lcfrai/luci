package deploymentkube

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// The closed client retains only its already resolved explicit Kubernetes
// credentials in memory. No browser URI, arbitrary destination or exec API is
// exposed. Static and request-scoped Entra clients use the same construction.
type storageForwardClient struct {
	kubernetes.Interface
	config *rest.Config
}
type storageContextTransport struct {
	context context.Context
	next    http.RoundTripper
}

func (t storageContextTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.next.RoundTrip(r.WithContext(t.context))
}
func WithStorageForwardClient(client kubernetes.Interface, config *rest.Config) kubernetes.Interface {
	if client == nil || config == nil {
		return client
	}
	return &storageForwardClient{Interface: client, config: rest.CopyConfig(config)}
}

type storageForwarder interface {
	storageForward(context.Context, string, string, int) (string, func() error, error)
}

func (c *storageForwardClient) storageForward(ctx context.Context, namespace, pod string, port int) (string, func() error, error) {
	if ctx.Err() != nil || namespace == "" || pod == "" || len(kubevalidation.IsDNS1123Label(namespace)) != 0 || len(kubevalidation.IsDNS1123Subdomain(pod)) != 0 || (port != 9090 && port != 3200) {
		return "", nil, ErrRefused
	}
	forwardContext, cancelForward := context.WithCancel(ctx)
	config := rest.CopyConfig(c.config)
	config.Timeout = 10 * time.Second
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		cancelForward()
		return "", nil, ErrRefused
	}
	endpoint, err := url.Parse(config.Host)
	if err != nil {
		cancelForward()
		return "", nil, ErrRefused
	}
	endpoint.Path = "/api/v1/namespaces/" + namespace + "/pods/" + pod + "/portforward"
	endpoint.RawQuery = ""
	// Kubernetes credentials authenticate only THIS API hop. The storage HTTP
	// client below has a different transport and only the reviewed app mTLS pair.
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: storageContextTransport{context: forwardContext, next: transport}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, http.MethodPost, endpoint)
	stop, ready := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	forward, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, []string{":" + strconv.Itoa(port)}, stop, ready, io.Discard, io.Discard)
	if err != nil {
		cancelForward()
		return "", nil, ErrRefused
	}
	var once sync.Once
	finished := false
	var result error
	closeTunnel := func() error {
		once.Do(func() { close(stop) })
		if !finished {
			select {
			case result = <-done:
				finished = true
			case <-time.After(2 * time.Second):
				cancelForward()
				select {
				case result = <-done:
					finished = true
				case <-time.After(2 * time.Second):
					return ErrRefused
				}
			}
		}
		cancelForward()
		if result != nil {
			return ErrRefused
		}
		return nil
	}
	go func() { done <- forward.ForwardPorts() }()
	select {
	case <-ready:
	case <-ctx.Done():
		_ = closeTunnel()
		return "", nil, ErrRefused
	case result = <-done:
		finished = true
		_ = closeTunnel()
		return "", nil, ErrRefused
	case <-time.After(5 * time.Second):
		_ = closeTunnel()
		return "", nil, ErrRefused
	}
	ports, err := forward.GetPorts()
	if err != nil || len(ports) != 1 || ports[0].Remote != uint16(port) || ports[0].Local == 0 {
		_ = closeTunnel()
		return "", nil, ErrRefused
	}
	// Caller MUST close normally and require the positive result, including on
	// request failure. No timer restarts the tunnel, worker or observation.
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(int(ports[0].Local))), closeTunnel, nil
}

func storageHTTPClient(ctx context.Context, target Target, pod, serverName string, port int, ca, cert, key []byte) (*http.Client, func() error, error) {
	source, ok := target.Client.(storageForwarder)
	if !ok || len(ca) == 0 || len(ca) > 256<<10 || len(cert) == 0 || len(cert) > 64<<10 || len(key) == 0 || len(key) > 64<<10 {
		return nil, nil, ErrRefused
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, nil, ErrRefused
	}
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, nil, ErrRefused
	}
	local, closeTunnel, err := source.storageForward(ctx, target.Namespace, pod, port)
	if err != nil {
		return nil, nil, ErrRefused
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, ForceAttemptHTTP2: false, TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 32 << 10, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName, Certificates: []tls.Certificate{pair}}, DialContext: func(call context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != net.JoinHostPort(serverName, strconv.Itoa(port)) {
			return nil, ErrRefused
		}
		return (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: -1}).DialContext(call, "tcp", local)
	}}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	closeAll := func() error {
		transport.CloseIdleConnections()
		if err := closeTunnel(); err != nil {
			return fmt.Errorf("owned storage query transport did not close normally")
		}
		return nil
	}
	return client, closeAll, nil
}
