package e2bbe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func azureTestClientCertificate(t *testing.T) *tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}, &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func TestAzureGatewayClientIdentityAndSeparateE2BHeaders(t *testing.T) {
	seen := 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) != 1 || r.Header.Get("X-API-Key") != "task-key" || r.Header.Get("X-Admin-Token") != "admin-key" {
			t.Error("gateway identity or separate provider grants were lost")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		seen++
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	_, withoutIdentity, err := privateHTTP(server.URL, roots, nil)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-API-Key", "task-key")
	request.Header.Set("X-Admin-Token", "admin-key")
	if response, err := withoutIdentity.Do(request); err == nil {
		response.Body.Close()
		t.Fatal("private gateway accepted a caller without its client certificate")
	}
	_, withIdentity, err := privateHTTP(server.URL, roots, azureTestClientCertificate(t))
	if err != nil {
		t.Fatal(err)
	}
	response, err := withIdentity.Do(request)
	if err != nil || response.StatusCode != http.StatusOK || seen != 1 {
		t.Fatal("client identity did not preserve separate provider authorization", err, seen)
	}
	response.Body.Close()
}
