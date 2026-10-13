// azure-gateway-config renders the private Envoy boundary for the pinned E2B
// Embed API and client proxy. It does not provision Azure or start a gateway.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

type config struct {
	GatewayHost     string `json:"gatewayHost"`
	PrivateListenIP string `json:"privateListenIp"`
	Scope           string `json:"scope"`
	Generation      string `json:"generation"`
	ServerCertFile  string `json:"serverCertFile"`
	ServerKeyFile   string `json:"serverKeyFile"`
	ClientCAFile    string `json:"clientCaFile"`
}

var hostPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{2,252}$`)
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,127}$`)
var pathPattern = regexp.MustCompile(`^/[A-Za-z0-9/._-]+$`)

func validConfig(c config) bool {
	ip := net.ParseIP(c.PrivateListenIP)
	if ip == nil || !ip.IsPrivate() || strings.Contains(c.GatewayHost, "..") ||
		!hostPattern.MatchString(c.GatewayHost) || !idPattern.MatchString(c.Scope) || !idPattern.MatchString(c.Generation) {
		return false
	}
	for _, path := range []string{c.ServerCertFile, c.ServerKeyFile, c.ClientCAFile} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || !pathPattern.MatchString(path) {
			return false
		}
	}
	return c.ServerCertFile != c.ServerKeyFile && c.ServerCertFile != c.ClientCAFile && c.ServerKeyFile != c.ClientCAFile
}

type allowedRoute struct{ method, path, auth, cluster string }

var managementRoutes = []allowedRoute{
	{"GET", `/nodes`, "x-admin-token", "e2b-api"},
	{"GET", `/templates`, "x-api-key", "e2b-api"},
	{"GET", `/templates/[a-z0-9_-]+/builds/[a-z0-9_-]+/status`, "x-api-key", "e2b-api"},
	{"GET", `/v2/templates`, "x-api-key", "e2b-api"},
	{"POST", `/v3/templates`, "x-api-key", "e2b-api"},
	{"GET", `/v2/templates/[a-z0-9_-]+/builds/[a-z0-9_-]+`, "x-api-key", "e2b-api"},
	{"POST", `/v2/templates/[a-z0-9_-]+/builds/[a-z0-9_-]+`, "x-api-key", "e2b-api"},
	{"GET", `/v2/sandboxes`, "x-api-key", "e2b-api"},
	{"POST", `/v2/sandboxes`, "x-api-key", "e2b-api"},
	{"GET", `/sandboxes/[a-z0-9]+`, "x-api-key", "e2b-api"},
	{"DELETE", `/sandboxes/[a-z0-9]+`, "x-api-key", "e2b-api"},
}

func writeRoute(out *bytes.Buffer, r allowedRoute, hostRegex string) {
	fmt.Fprintf(out, "            - match:\n                safe_regex: {google_re2: {}, regex: '^%s$'}\n                headers:\n", r.path)
	fmt.Fprintf(out, "                  - name: ':method'\n                    string_match: {exact: '%s'}\n", r.method)
	if hostRegex != "" {
		fmt.Fprintf(out, "                  - name: ':authority'\n                    string_match:\n                      safe_regex: {google_re2: {}, regex: '%s'}\n", hostRegex)
	}
	fmt.Fprintf(out, "                  - name: '%s'\n                    present_match: true\n", r.auth)
	if r.cluster == "e2b-api" {
		other := "x-admin-token"
		if r.auth == other {
			other = "x-api-key"
		}
		fmt.Fprintf(out, "                  - name: '%s'\n                    present_match: true\n                    invert_match: true\n", other)
	} else {
		fmt.Fprint(out, "                  - name: 'e2b-traffic-access-token'\n                    present_match: true\n")
		for _, other := range []string{"x-api-key", "x-admin-token"} {
			fmt.Fprintf(out, "                  - name: '%s'\n                    present_match: true\n                    invert_match: true\n", other)
		}
	}
	timeout := "15s"
	if strings.Contains(r.path, "Start") || r.path == "/attach" {
		// Envd process Start is a streaming response; sandboxd attach upgrades
		// to a WebSocket. The task's own bounded deadline and owned close
		// control their lifetime, not Envoy's ordinary HTTP response timer.
		timeout = "0s"
	}
	fmt.Fprintf(out, "              route: {cluster: %s, timeout: %s}\n", r.cluster, timeout)
}

func render(c config) ([]byte, error) {
	if !validConfig(c) {
		return nil, errors.New("private gateway configuration is invalid")
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, `# Generated for one private controller generation. Deny all unmatched routes.
static_resources:
  listeners:
  - name: luci_private_gateway
    address: {socket_address: {address: '%s', port_value: 443}}
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: luci_embed
          codec_type: AUTO
          forward_client_cert_details: SANITIZE
          upgrade_configs: [{upgrade_type: websocket}]
          route_config:
            name: exact_luci_embed_routes
            virtual_hosts:
            - name: management
              domains: ['%s']
              routes:
`, c.PrivateListenIP, c.GatewayHost)
	for _, r := range managementRoutes {
		writeRoute(&out, r, "")
	}
	// Only the mTLS controller can read this nonsecret deployment marker. It
	// binds the route generation independently of the E2B task/admin keys.
	fmt.Fprintf(&out, "            - match: {path: '/luci-gateway/receipt', headers: [{name: ':method', string_match: {exact: 'GET'}}]}\n              direct_response: {status: 200, body: {inline_string: '%s'}}\n", e2bbe.GatewayReceiptDigest(c.GatewayHost, c.Scope, c.Generation))
	fmt.Fprintf(&out, "            - name: owned_guest\n              domains: ['*.%s']\n              routes:\n", c.GatewayHost)
	guestHost := regexp.QuoteMeta(c.GatewayHost)
	writeRoute(&out, allowedRoute{"GET", `/files`, "x-access-token", "e2b-client-proxy"}, "^49983-[a-z0-9]+\\."+guestHost+"$")
	writeRoute(&out, allowedRoute{"POST", `/files`, "x-access-token", "e2b-client-proxy"}, "^49983-[a-z0-9]+\\."+guestHost+"$")
	writeRoute(&out, allowedRoute{"POST", `/process\.Process/(Start|List)`, "x-access-token", "e2b-client-proxy"}, "^49983-[a-z0-9]+\\."+guestHost+"$")
	writeRoute(&out, allowedRoute{"GET", `/attach`, "authorization", "e2b-client-proxy"}, "^8080-[a-z0-9]+\\."+guestHost+"$")
	fmt.Fprintf(&out, `          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              '@type': type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
      transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          '@type': type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
          require_client_certificate: true
          common_tls_context:
            tls_certificates:
            - certificate_chain: {filename: '%s'}
              private_key: {filename: '%s'}
            validation_context:
              trusted_ca: {filename: '%s'}
              match_typed_subject_alt_names:
              - san_type: URI
                matcher: {exact: 'spiffe://luci/controller/%s/%s'}
  clusters:
  - name: e2b-api
    type: STATIC
    connect_timeout: 2s
    load_assignment:
      cluster_name: e2b-api
      endpoints: [{lb_endpoints: [{endpoint: {address: {socket_address: {address: 127.0.0.1, port_value: 3000}}}}]}]
  - name: e2b-client-proxy
    type: STATIC
    connect_timeout: 2s
    load_assignment:
      cluster_name: e2b-client-proxy
      endpoints: [{lb_endpoints: [{endpoint: {address: {socket_address: {address: 127.0.0.1, port_value: 3002}}}}]}]
`, c.ServerCertFile, c.ServerKeyFile, c.ClientCAFile, c.Scope, c.Generation)
	return out.Bytes(), nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: azure-gateway-config PRIVATE_CONFIG.json")
		os.Exit(2)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "private gateway configuration unavailable")
		os.Exit(2)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		fmt.Fprintln(os.Stderr, "gateway configuration must be a private regular file")
		os.Exit(2)
	}
	var c config
	decoder := json.NewDecoder(io.LimitReader(file, 4097))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&c) != nil || decoder.Decode(new(any)) != io.EOF {
		fmt.Fprintln(os.Stderr, "gateway configuration is invalid")
		os.Exit(2)
	}
	data, err := render(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	_, _ = os.Stdout.Write(data)
}
