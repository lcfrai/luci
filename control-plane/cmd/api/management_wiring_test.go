package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bootstrapServingTest(t *testing.T, cfg Config, addr string, client *http.Client) string {
	t.Helper()
	root := cfg.StateDir
	if root == "" {
		root = filepath.Dir(filepath.Dir(cfg.SessionsPath))
	}
	code, err := os.ReadFile(filepath.Join(root, "control-plane", "bootstrap-token"))
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"bootstrapToken": strings.TrimSpace(string(code)), "username": "owner-test", "displayName": "Test Owner", "password": "test-password-long-enough"})
	res, err := client.Post("http://"+addr+"/v1/auth/bootstrap", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var reply struct {
		Token string `json:"token"`
	}
	if res.StatusCode != 201 || json.NewDecoder(res.Body).Decode(&reply) != nil || reply.Token == "" {
		t.Fatal("bootstrap", res.StatusCode)
	}
	return reply.Token
}

func TestManagedShutdownNeedsOwnedPrivateToken(t *testing.T) {
	called := make(chan struct{}, 1)
	token := strings.Repeat("x", 40)
	handler := withManagedShutdown(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Luci-Lifecycle-Token") != "" {
			t.Error("lifecycle token leaked intoapplication")
		}
		w.WriteHeader(204)
	}), token, func() { called <- struct{}{} })
	for _, x := range []struct {
		method, path, token, remote string
		want                        int
	}{{"POST", "/v1/control-plane/shutdown", "", "127.0.0.1:4444", 403}, {"POST", "/v1/control-plane/shutdown", token, "192.0.2.1:4444", 403}, {"POST", "/v1/control-plane/shutdown", token, "127.0.0.1:4444", 200}, {"GET", "/v1/setup/status", token, "127.0.0.1:4444", 204}} {
		r := httptest.NewRequest(x.method, x.path, nil)
		r.RemoteAddr = x.remote
		r.Header.Set("X-Luci-Lifecycle-Token", x.token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != x.want {
			t.Fatal(x, w.Code)
		}
	}
	<-called
}
