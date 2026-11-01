package e2bbe

import (
	"crypto/x509"
	"net/http"
	"testing"
)

func TestAzureInstanceTransportKeepsTLSNameAndRefusesPublicRoute(t *testing.T) {
	certificate := azureTestClientCertificate(t)
	roots := x509.NewCertPool()
	u, client, err := privateHTTP("https://gateway.private.internal", roots, certificate, "10.0.0.7")
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	transport := client.Transport.(*http.Transport)
	if u.Host != "gateway.private.internal" || transport.TLSClientConfig.InsecureSkipVerify || len(transport.TLSClientConfig.Certificates) != 1 || transport.Proxy != nil {
		t.Fatal("per-instance routing weakened original TLS or client identity")
	}
	if _, _, err = privateHTTP("https://gateway.private.internal", roots, certificate, "8.8.8.8"); err == nil {
		t.Fatal("public IP admitted as a private host")
	}
	if _, _, err = privateHTTP("https://gateway.private.internal", roots, nil, "10.0.0.7"); err == nil {
		t.Fatal("instance pin bypassed mutual TLS")
	}
}
