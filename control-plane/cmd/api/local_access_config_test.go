package main

import (
	"strings"
	"testing"
)

func TestLocalAccessCapabilityRequiresExplicitLoopbackAndNeverWorkers(t *testing.T) {
	secret := strings.Repeat("x", 43)
	for _, test := range []struct {
		addr, worker, token string
		valid               bool
	}{
		{"127.0.0.1:0", "", secret, true}, {"[::1]:0", "", secret, true},
		{"0.0.0.0:8730", "", secret, false}, {":8730", "", secret, false}, {"localhost:8730", "", secret, false},
		{"127.0.0.1:8730", "1", secret, false}, {"127.0.0.1:8730", "", "short", false},
		{"127.0.0.1:8730", "", secret + "\n", false},
	} {
		cfg, err := loadConfig(env(map[string]string{envStateDir: t.TempDir(), envAddr: test.addr, envWorker: test.worker, envLocalAccessToken: test.token, envAllowPublicBind: "1"}))
		if (err == nil) != test.valid {
			t.Errorf("addr%s worker%s valid%v goterror%v", test.addr, test.worker, test.valid, err)
		}
		if test.valid && cfg.LocalAccessToken != secret {
			t.Fatal("native capability lost")
		}
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Fatal("capability leaked into diagnostics")
		}
	}
}

func TestPortableOwnerAccessRequiresExplicitPrivateLoopbackAndNeverNativeOrWorker(t *testing.T) {
	for _, test := range []struct {
		name, addr, public, worker, native string
		valid                              bool
	}{
		{"ipv4", "127.0.0.1:0", "", "", "", true},
		{"ipv6", "[::1]:0", "", "", "", true},
		{"public address", "0.0.0.0:8730", "", "", "", false},
		{"hostname", "localhost:8730", "", "", "", false},
		{"public opt in", "127.0.0.1:8730", "1", "", "", false},
		{"worker", "127.0.0.1:8730", "", "1", "", false},
		{"native", "127.0.0.1:8730", "", "", strings.Repeat("n", 43), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := loadConfig(env(map[string]string{envStateDir: t.TempDir(), envAddr: test.addr, envPortableOwnerAccess: "1", envAllowPublicBind: test.public, envWorker: test.worker, envLocalAccessToken: test.native}))
			if (err == nil) != test.valid || (test.valid && !cfg.PortableOwnerAccess) {
				t.Fatalf("portable configuration valid=%v, cfg=%v, err=%v", test.valid, cfg.PortableOwnerAccess, err)
			}
		})
	}
}
