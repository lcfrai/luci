package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixtureAllowlist() allowlist {
	return allowlist{
		Version: 1, Repository: repository,
		ManifestDigest: "sha256:" + strings.Repeat("a", 64), ManifestBytes: 4,
		ConfigDigest: "sha256:" + strings.Repeat("b", 64), ConfigBytes: 5,
		Layers: []blob{{Digest: "sha256:" + strings.Repeat("c", 64), Bytes: 6}},
	}
}

func TestAllowlistExactBytesAndBounds(t *testing.T) {
	a := fixtureAllowlist()
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if _, err := decodeAllowlist(data, hex.EncodeToString(hash[:])); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		name string
		data []byte
		hash string
	}{
		{"wrong-hash", data, strings.Repeat("0", 64)},
		{"unknown-field", append(bytes.TrimSuffix(data, []byte("}")), []byte(`,"extra":true}`)...), ""},
		{"duplicate-layer", nil, ""},
	} {
		if bad.name == "duplicate-layer" {
			a.Layers = append(a.Layers, a.Layers[0])
			bad.data, _ = json.Marshal(a)
		}
		if bad.hash == "" {
			h := sha256.Sum256(bad.data)
			bad.hash = hex.EncodeToString(h[:])
		}
		if _, err := decodeAllowlist(bad.data, bad.hash); err == nil {
			t.Errorf("accepted %s", bad.name)
		}
	}
}

func TestFixedReadOnlyGate(t *testing.T) {
	a := fixtureAllowlist()
	backendCalls := 0
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		backendCalls++
		if r.URL.Host != backendAddress || r.URL.Path != "/v2/"+repository+"/blobs/"+a.ConfigDigest || r.Host != backendAddress || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected backend request: %v", r)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/octet-stream"}}, ContentLength: 5, Body: io.NopCloser(strings.NewReader("hello"))}, nil
	})}
	gate := newGate(a, client)
	for _, tc := range []struct {
		method, path string
		headers      map[string]string
		want         int
	}{
		{http.MethodGet, "/v2/", nil, 200},
		{http.MethodGet, "/v2/" + repository + "/blobs/" + a.ConfigDigest, nil, 200},
		{http.MethodPost, "/v2/" + repository + "/blobs/uploads/", nil, 405},
		{http.MethodGet, "/v2/other/blobs/" + a.ConfigDigest, nil, 404},
		{http.MethodGet, "/v2/" + repository + "/blobs/" + a.ConfigDigest + "?extra=1", nil, 404},
		{http.MethodGet, "/v2/" + repository + "/blobs/" + a.ConfigDigest, map[string]string{"Authorization": "secret"}, 404},
		{http.MethodGet, "/v2/" + repository + "/blobs/" + a.ConfigDigest, map[string]string{"Range": "bytes=5-"}, 416},
	} {
		req := httptest.NewRequest(tc.method, "http://127.0.0.1:5000"+tc.path, nil)
		// net/http servers receive origin-form URLs; httptest constructs an
		// absolute URL here, which the gate correctly rejects as proxy input.
		req.URL.Scheme, req.URL.Host = "", ""
		for name, value := range tc.headers {
			req.Header.Set(name, value)
		}
		out := httptest.NewRecorder()
		gate.ServeHTTP(out, req)
		if out.Code != tc.want {
			t.Errorf("%s %s: got %d want %d", tc.method, tc.path, out.Code, tc.want)
		}
	}
	if backendCalls != 1 {
		t.Errorf("backend calls=%d, want 1", backendCalls)
	}
}

func TestBackendRedirectRefused(t *testing.T) {
	a := fixtureAllowlist()
	gate := newGate(a, &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": {"http://elsewhere.invalid"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:5000/v2/"+repository+"/blobs/"+a.ConfigDigest, nil)
	req.URL.Scheme, req.URL.Host = "", ""
	out := httptest.NewRecorder()
	gate.ServeHTTP(out, req)
	if out.Code != http.StatusBadGateway {
		t.Fatalf("redirect status=%d", out.Code)
	}
}

func TestRegistryConfigReadOnlyLocal(t *testing.T) {
	config := registryConfig()
	for _, want := range []string{"rootdirectory: /registry-data", "readonly:\n      enabled: true", "addr: 127.0.0.1:5001", "uploadpurging:\n      enabled: false"} {
		if !strings.Contains(config, want) {
			t.Errorf("config misses %q", want)
		}
	}
	if strings.Contains(config, "redirect:") || strings.Contains(config, "s3:") {
		t.Fatal("unexpected remote storage or redirect")
	}
}
