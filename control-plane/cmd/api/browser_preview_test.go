package main

import (
	"strings"
	"testing"
)

func TestBrowserGatewayConfigurationIsExplicitAndSeparate(t *testing.T) {
	state := t.TempDir()
	native, err := loadConfig(env(map[string]string{envStateDir: state}))
	if err != nil {
		t.Fatal(err)
	}
	if native.BrowserPreviewAddr != "" || native.BrowserPreviewPort != 0 || strings.Contains(webCSP(native), "*.localhost") {
		t.Fatal("native portable default changed")
	}
	for _, values := range []map[string]string{
		{envBrowserPreviewPort: "8731"},
		{envBrowserPreviewAddr: "0.0.0.0:8731", envBrowserPreviewPort: "8731"},
		{envBrowserPreviewAddr: "127.0.0.1:8730", envBrowserPreviewPort: "8731"},
		{envBrowserPreviewAddr: "127.0.0.1:8731", envBrowserPreviewPort: "65536"},
		{envWorker: "1", envBrowserPreviewAddr: "127.0.0.1:8731", envBrowserPreviewPort: "8731"},
	} {
		values[envStateDir] = state
		if _, err := loadConfig(env(values)); err == nil {
			t.Fatalf("unsafe gateway configuration accepted: %#v", values)
		}
	}
	docker, err := loadConfig(env(map[string]string{envStateDir: state, envAddr: "0.0.0.0:8730", envAllowPublicBind: "1", envBrowserPreviewAddr: "0.0.0.0:8731", envBrowserPreviewPort: "9876"}))
	if err != nil {
		t.Fatal(err)
	}
	if docker.BrowserPreviewPort != 9876 || !strings.Contains(webCSP(docker), "http://*.localhost:9876") || !strings.Contains(webCSP(docker), "ws://*.localhost:9876") {
		t.Fatal("advertised external port not reflected in browser policy")
	}
	worker, err := loadConfig(env(map[string]string{envStateDir: state, envWorker: "1", envAddr: "127.0.0.1:0", envBrowserPreviewPort: "9876"}))
	if err != nil || worker.BrowserPreviewAddr != "" || worker.BrowserPreviewPort != 9876 {
		t.Fatal("worker cannot register without opening its own public listener")
	}
}
