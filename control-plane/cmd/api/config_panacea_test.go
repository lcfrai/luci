package main

// Phase 7's three variables. They are in a file of their own because
// config_test.go is phase 2's and its rows are a contract 4 table; these are
// contract 4's three new rows and they arrive together.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
)

// TestConfigPanaceaURL: a bad address stops the process at startup, beside the
// variable that caused it. The path case is the measured one - a doubled /api
// produced a 404 nobody could explain from the answer.
func TestConfigPanaceaURL(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ value, want string }{
		"a path": {"https://dev.panacea.kpmg/api",
			"Give the address without a path"},
		"plain http": {"http://dev.panacea.kpmg",
			"must start with https://"},
		"not an address": {"dev.panacea.kpmg", "must start with https://"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := loadConfig(env(map[string]string{
				"PANACEA_APP_STATE_DIR":    t.TempDir(),
				"PANACEA_APP_PANACEA_URL":  tc.value,
				"PANACEA_APP_PROJECTS_DIR": "",
			}))
			if err == nil {
				t.Fatalf("loadConfig accepted PANACEA_APP_PANACEA_URL=%q", tc.value)
			}
			if !strings.Contains(err.Error(), "PANACEA_APP_PANACEA_URL") {
				t.Errorf("error %q does not name the variable", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not carry %q", err, tc.want)
			}
		})
	}
}

// TestConfigPanaceaDefaults: unset means the user chooses the address, and the
// timeout is the client's own default rather than a second number here.
func TestConfigPanaceaDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg, err := loadConfig(env(map[string]string{"PANACEA_APP_STATE_DIR": dir}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.PanaceaURL != "" || cfg.PanaceaCAFile != "" {
		t.Errorf("PanaceaURL %q, PanaceaCAFile %q; want both empty", cfg.PanaceaURL, cfg.PanaceaCAFile)
	}
	if cfg.PanaceaTimeout != panacea.DefaultTimeout {
		t.Errorf("PanaceaTimeout = %v, want %v", cfg.PanaceaTimeout, panacea.DefaultTimeout)
	}
	if want := filepath.Join(dir, "deploy"); cfg.DeployStateDir != want {
		t.Errorf("DeployStateDir = %q, want %q", cfg.DeployStateDir, want)
	}
}

// TestConfigPanaceaAccepted: the address is canonicalised here, once, so
// nothing downstream compares two spellings of one host.
func TestConfigPanaceaAccepted(t *testing.T) {
	t.Parallel()
	cfg, err := loadConfig(env(map[string]string{
		"PANACEA_APP_STATE_DIR":       t.TempDir(),
		"PANACEA_APP_PANACEA_URL":     "HTTPS://Dev.Panacea.KPMG/",
		"PANACEA_APP_PANACEA_TIMEOUT": "45s",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.PanaceaURL != "https://dev.panacea.kpmg" {
		t.Errorf("PanaceaURL = %q, want the canonical form", cfg.PanaceaURL)
	}
	if cfg.PanaceaTimeout != 45*time.Second {
		t.Errorf("PanaceaTimeout = %v, want 45s", cfg.PanaceaTimeout)
	}
}

// TestConfigPanaceaCAFile: the same rule as the sandbox bundle, its own
// sentences. A file that holds a private key and no certificate trusts nothing,
// and ignoring it would leave every Panacea call failing its handshake with an
// error that names neither the file nor the variable.
func TestConfigPanaceaCAFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	notACert := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(notACert, []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	for name, tc := range map[string]struct{ value, want string }{
		"no certificate": {notACert, "no certificate in " + notACert},
		"missing":        {filepath.Join(dir, "absent.pem"), "no such file"},
		"relative":       {"ca.pem", "is not an absolute path"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := loadConfig(env(map[string]string{
				"PANACEA_APP_STATE_DIR":       t.TempDir(),
				"PANACEA_APP_PANACEA_CA_FILE": tc.value,
			}))
			if err == nil {
				t.Fatalf("loadConfig accepted PANACEA_APP_PANACEA_CA_FILE=%q", tc.value)
			}
			if !strings.Contains(err.Error(), "PANACEA_APP_PANACEA_CA_FILE") ||
				!strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name the variable and carry %q", err, tc.want)
			}
		})
	}
}
