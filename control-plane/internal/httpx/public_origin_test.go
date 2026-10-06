package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicOriginExactHostAndCORSNeverTrustForwarding(t *testing.T) {
	policy := NewOriginPolicy([]string{"https://app.example.test"}, nil)
	if err := policy.SetPublicAppOrigin("https://app.example.test"); err != nil {
		t.Fatal(err)
	}
	h := policy.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) }))
	for _, tc := range []struct {
		host, origin string
		want         int
	}{{"app.example.test", "https://app.example.test", 201}, {"app.example.test", "https://other.example.test", 403}, {"p-any.preview.example.test", "https://app.example.test", 403}, {"app.example.test:443", "https://app.example.test", 403}, {"attacker.test", "https://app.example.test", 403}, {"127.0.0.1:8730", "https://app.example.test", 201}} {
		r := httptest.NewRequest("POST", "http://"+tc.host+"/v1/auth/login", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-Forwarded-Host", "app.example.test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(tc, w.Code)
		}
	}
}
