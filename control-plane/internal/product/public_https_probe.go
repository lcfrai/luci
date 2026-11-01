package product

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"time"
)

type publicHTTPSProbeResult struct {
	DNSMatched, HTTPSVerified              bool
	CertificateNotAfter, CertificateSHA256 string
	HTTPStatus                             int
}
type publicHTTPSProbeFunc func(context.Context, string, string, string) (publicHTTPSProbeResult, error)

func publicHTTPSIPv4(value string) bool {
	ip, e := netip.ParseAddr(value)
	return e == nil && ip.Is4() && publicResearchIP(ip)
}

// No body, arbitrary destination, proxy, redirect or ambient credential is used.
// A valid chain/SNI name proves TLS, not application business health or success.
func probePublicHTTPS(ctx context.Context, hostname, address, path string) (publicHTTPSProbeResult, error) {
	result := publicHTTPSProbeResult{}
	if !publicHTTPSIPv4(address) || hostname == "" || !publicHTTPSPathGrammar.MatchString(path) {
		return result, ErrPublicHTTPS
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	values, e := net.DefaultResolver.LookupNetIP(ctx, "ip", hostname)
	if e != nil || len(values) == 0 || len(values) > 8 {
		return result, ErrPublicHTTPS
	}
	for _, ip := range values {
		if !ip.Unmap().Is4() || ip.Unmap().String() != address {
			return result, ErrPublicHTTPS
		}
	}
	result.DNSMatched = true
	dialer := net.Dialer{Timeout: 4 * time.Second}
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hostname}, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 4 * time.Second, MaxResponseHeaderBytes: 32 << 10, DisableCompression: true, DisableKeepAlives: true, DialContext: func(c context.Context, network, remote string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(remote)
		if e != nil || host != hostname || port != "443" || network != "tcp" {
			return nil, errors.New("Unexpected verification destination")
		}
		return dialer.DialContext(c, "tcp", net.JoinHostPort(address, "443"))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, e := http.NewRequestWithContext(ctx, http.MethodHead, "https://"+hostname+path, nil)
	if e != nil {
		return result, ErrPublicHTTPS
	}
	request.Header.Set("User-Agent", "Luci-HTTPS-verification/1")
	response, e := client.Do(request)
	if e != nil {
		return result, ErrPublicHTTPS
	}
	closeErr := response.Body.Close()
	if closeErr != nil || ctx.Err() != nil || response.TLS == nil || len(response.TLS.VerifiedChains) == 0 || len(response.TLS.PeerCertificates) == 0 || response.StatusCode < 100 || response.StatusCode > 599 {
		return result, ErrPublicHTTPS
	}
	leaf := response.TLS.PeerCertificates[0]
	sum := sha256.Sum256(leaf.Raw)
	result.HTTPSVerified = true
	result.CertificateNotAfter = leaf.NotAfter.UTC().Format(time.RFC3339Nano)
	result.CertificateSHA256 = hex.EncodeToString(sum[:])
	result.HTTPStatus = response.StatusCode
	// Recheck the resolver after the handshake; this remains a sampled observation,
	// not an atomic guarantee against a later DNS/provider/network change.
	latest, e := net.DefaultResolver.LookupNetIP(ctx, "ip", hostname)
	if e != nil || len(latest) == 0 || len(latest) > 8 {
		return publicHTTPSProbeResult{}, ErrPublicHTTPS
	}
	for _, ip := range latest {
		if ip.Unmap().String() != address {
			return publicHTTPSProbeResult{}, ErrPublicHTTPS
		}
	}
	return result, nil
}
